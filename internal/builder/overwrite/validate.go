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

package overwrite

import (
	"encoding/json"
	"fmt"

	"github.com/chideat/valkey-operator/api/core"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kjson "sigs.k8s.io/json"
)

// Validate checks the overwrites of component the way admission does: a
// supported kind, one entry per kind, and a patch that decodes strictly into
// that kind, uses no patch directive, names only containers the operator runs
// and sets no protected field.
func Validate(overwrites []core.Overwrite, component Component, fldPath *field.Path) field.ErrorList {
	var (
		errs field.ErrorList
		seen = map[core.OverwriteKind]bool{}
	)
	for i, ow := range overwrites {
		p := fldPath.Index(i)
		var (
			schema any
			guards []guard
		)
		switch ow.Kind {
		case core.OverwriteKindStatefulSet:
			schema, guards = &appsv1.StatefulSet{}, statefulSetGuards(component)
		default:
			errs = append(errs, field.NotSupported(p.Child("kind"), ow.Kind, []core.OverwriteKind{core.OverwriteKindStatefulSet}))
			continue
		}
		if seen[ow.Kind] {
			errs = append(errs, field.Duplicate(p.Child("kind"), ow.Kind))
			continue
		}
		seen[ow.Kind] = true
		errs = append(errs, validatePatch(ow.Patch.Raw, schema, guards, p.Child("patch"))...)
	}
	return errs
}

func validatePatch(raw []byte, schema any, guards []guard, fldPath *field.Path) field.ErrorList {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		return field.ErrorList{field.Invalid(fldPath, string(raw), "must be a JSON object")}
	}
	if key := directiveIn(doc); key != "" {
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("patch directive %s is not supported", key))}
	}

	// Strict decoding is case-sensitive and rejects unknown fields; a field
	// the API does not know would otherwise be dropped without a word.
	strictErrs, err := kjson.UnmarshalStrict(raw, schema)
	if err != nil {
		return field.ErrorList{field.Invalid(fldPath, string(raw), err.Error())}
	}
	var errs field.ErrorList
	for _, e := range strictErrs {
		errs = append(errs, field.Forbidden(fldPath, e.Error()))
	}

	report := func(at location, msg string) {
		errs = append(errs, field.Forbidden(fldPath, fmt.Sprintf("%s %s", at, msg)))
	}
	for _, g := range guards {
		g.check(doc, nil, report)
	}
	return errs
}
