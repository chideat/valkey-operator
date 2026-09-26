/*
Copyright 2024 chideat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package overwrite merges spec.overwrites into the objects the builders
// generate. User values win, except for the fields the operator protects: the
// builders restore those, and admission rejects patches that set them. One set
// of rules serves both.
package overwrite

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	"github.com/chideat/valkey-operator/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/yaml"
)

// ChecksumAnnotation is set on every object that has overwrites merged in, to
// a hash of them. The actors compare it on the generated and the live object,
// so adding, changing or removing an overwrite is noticed: for StatefulSets
// through their full annotation comparison, for other kinds through
// ChecksumChanged.
var ChecksumAnnotation = builder.ChecksumKey("overwrites")

// StatefulSet merges the StatefulSet overwrites into sts, a StatefulSet
// generated for component. Protected fields keep their generated value, and a
// patch that cannot be merged is skipped; problems lists both, for a Warning
// event. Without StatefulSet overwrites sts is returned as it is.
func StatefulSet(sts *appsv1.StatefulSet, overwrites []core.Overwrite, component Component) (*appsv1.StatefulSet, []string, error) {
	return apply(sts, overwrites, core.OverwriteKindStatefulSet, statefulSetGuards(component))
}

// PodDisruptionBudget merges the PodDisruptionBudget overwrites into pdb, as
// StatefulSet does for StatefulSets. Without PodDisruptionBudget overwrites pdb
// is returned as it is.
func PodDisruptionBudget(pdb *policyv1.PodDisruptionBudget, overwrites []core.Overwrite) (*policyv1.PodDisruptionBudget, []string, error) {
	return apply(pdb, overwrites, core.OverwriteKindPodDisruptionBudget, podDisruptionBudgetGuards())
}

// ChecksumChanged reports whether the overwrites merged into generated differ
// from those in live: one was added, changed or removed. Objects with no
// overwrites on either side are unchanged.
func ChecksumChanged(generated, live metav1.Object) bool {
	return generated.GetAnnotations()[ChecksumAnnotation] != live.GetAnnotations()[ChecksumAnnotation]
}

// Report sends the problems of a merge into obj as one Warning event.
func Report(inst types.Instance, obj string, problems []string) {
	if len(problems) == 0 {
		return
	}
	inst.SendEventf(corev1.EventTypeWarning, config.EventOverwrites,
		"overwrites for %s: %s", obj, strings.Join(problems, "; "))
}

// patch is one overwrite of a kind: its position in spec.overwrites, and
// its document as JSON, or the error that kept it from being read.
type patch struct {
	index int
	raw   []byte
	err   error
}

func patchesOf(overwrites []core.Overwrite, kind core.OverwriteKind) []patch {
	var ret []patch
	for i, ow := range overwrites {
		if ow.Kind == kind {
			raw, err := document(ow.Patch.Raw)
			if err != nil {
				raw = ow.Patch.Raw
			}
			ret = append(ret, patch{index: i, raw: raw, err: err})
		}
	}
	return ret
}

// document returns the JSON object of a patch given as an object, or as a
// string that holds one in YAML or JSON. The string form exists for null:
// client-side kubectl apply and merge patches drop nulls from an object, but
// cannot see into a string.
func document(raw []byte) ([]byte, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		doc, err := yaml.YAMLToJSON([]byte(text))
		if err != nil {
			return nil, fmt.Errorf("patch text is not valid YAML: %w", err)
		}
		raw = doc
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, errors.New("patch must be an object, or a string that holds one in YAML or JSON")
	}
	return raw, nil
}

func apply[T any, PT interface {
	*T
	metav1.Object
}](obj PT, overwrites []core.Overwrite, kind core.OverwriteKind, guards []guard) (PT, []string, error) {
	patches := patchesOf(overwrites, kind)
	if len(patches) == 0 {
		return obj, nil, nil
	}

	base, err := json.Marshal(obj)
	if err != nil {
		return nil, nil, err
	}

	var (
		schema   T
		merged   = base
		problems []string
	)
	for _, p := range patches {
		if p.err != nil {
			problems = append(problems, fmt.Sprintf("overwrites[%d] skipped: %v", p.index, p.err))
			continue
		}
		if key := findDirective(p.raw); key != "" {
			problems = append(problems, fmt.Sprintf("overwrites[%d] skipped: patch directive %s is not supported", p.index, key))
			continue
		}
		next, err := strategicpatch.StrategicMergePatch(merged, p.raw, schema)
		if err == nil {
			// Decoding catches a value of the wrong type before it can fail
			// the whole object.
			err = json.Unmarshal(next, new(T))
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("overwrites[%d] skipped: %v", p.index, err))
			continue
		}
		merged = next
	}

	var baseObj, mergedObj map[string]any
	if err := json.Unmarshal(base, &baseObj); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(merged, &mergedObj); err != nil {
		return nil, nil, err
	}
	report := func(at location, msg string) {
		problems = append(problems, fmt.Sprintf("%s %s", at, msg))
	}
	for _, g := range guards {
		g.restore(baseObj, mergedObj, nil, report)
	}

	data, err := json.Marshal(mergedObj)
	if err != nil {
		return nil, nil, err
	}
	out := PT(new(T))
	if err := json.Unmarshal(data, out); err != nil {
		return nil, nil, err
	}

	sum, err := checksum(patches)
	if err != nil {
		return nil, nil, err
	}
	annotations := out.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[ChecksumAnnotation] = sum
	out.SetAnnotations(annotations)
	return out, problems, nil
}

// checksum hashes the patches in a canonical form: decoded and encoded again,
// which sorts map keys, so layout, key order, and giving a patch as text or as
// an object do not change it.
func checksum(patches []patch) (string, error) {
	docs := make([]any, 0, len(patches))
	for _, p := range patches {
		var doc any
		if err := json.Unmarshal(p.raw, &doc); err != nil {
			doc = string(p.raw)
		}
		docs = append(docs, doc)
	}
	data, err := json.Marshal(docs)
	if err != nil {
		return "", err
	}
	return util.GenerateObjectSig(data, "")
}

// findDirective returns the first key of a strategic merge patch directive,
// such as $patch or $setElementOrder, found in raw.
func findDirective(raw []byte) string {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return directiveIn(doc)
}

func directiveIn(v any) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.HasPrefix(k, "$") {
				return k
			}
			if d := directiveIn(t[k]); d != "" {
				return d
			}
		}
	case []any:
		for _, item := range t {
			if d := directiveIn(item); d != "" {
				return d
			}
		}
	}
	return ""
}
