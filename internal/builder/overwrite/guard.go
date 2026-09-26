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
	"fmt"
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
)

// A guard protects one part of a generated object. It works on objects
// decoded from JSON into map[string]any, so the same guard serves the builders,
// which restore a merged object, and admission, which checks a patch.
type guard interface {
	// restore makes merged match base in the guarded part and reports every
	// location it had to change.
	restore(base, merged map[string]any, at location, report reporter)
	// check reports every location in the guarded part that patch writes to,
	// including a null that would delete it.
	check(patch map[string]any, at location, report reporter)
}

// reporter receives a location and what happened there, or why it is refused.
type reporter func(at location, msg string)

// location is where a value sits in an object, as it is reported: map keys
// joined by dots, list items as [mergeKey=value], map entries as [key].
type location []string

func (l location) String() string {
	var b strings.Builder
	for i, s := range l {
		if i > 0 && !strings.HasPrefix(s, "[") {
			b.WriteByte('.')
		}
		b.WriteString(s)
	}
	return b.String()
}

func (l location) child(keys ...string) location {
	return append(slices.Clone(l), keys...)
}

func (l location) entry(key string) location {
	return l.child("[" + key + "]")
}

func (l location) item(mergeKey, id string) location {
	return l.child(fmt.Sprintf("[%s=%s]", mergeKey, id))
}

// lookup returns the value under keys and whether every key was present.
func lookup(obj map[string]any, keys []string) (any, bool) {
	var cur any = obj
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// ensure returns the map under keys, creating maps along the way and
// replacing anything that is not a map.
func ensure(obj map[string]any, keys []string) map[string]any {
	cur := obj
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[k] = next
		}
		cur = next
	}
	return cur
}

// walk follows keys through a patch. It returns the value at the end, whether
// it is present, and whether a null on the way deletes everything below it;
// nullAt is then the location of that null.
func walk(patch map[string]any, keys []string, at location) (val any, present bool, nullAt location) {
	var cur any = patch
	for i, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		if cur, ok = m[k]; !ok {
			return nil, false, nil
		}
		if cur == nil && i < len(keys)-1 {
			return nil, false, at.child(keys[:i+1]...)
		}
	}
	return cur, true, nil
}

func deepCopy(v any) any {
	return runtime.DeepCopyJSONValue(v)
}

// fixed guards the value under its keys: it stays as generated.
type fixed []string

func (f fixed) restore(base, merged map[string]any, at location, report reporter) {
	want, has := lookup(base, f)
	got, set := lookup(merged, f)
	if has == set && reflect.DeepEqual(want, got) {
		return
	}
	parent, key := f[:len(f)-1], f[len(f)-1]
	if has {
		ensure(merged, parent)[key] = deepCopy(want)
		report(at.child(f...), "restored")
		return
	}
	if m, ok := lookupMap(merged, parent); ok {
		delete(m, key)
	}
	report(at.child(f...), "removed")
}

func (f fixed) check(patch map[string]any, at location, report reporter) {
	_, present, nullAt := walk(patch, f, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
	case present:
		report(at.child(f...), "is protected")
	}
}

func lookupMap(obj map[string]any, keys []string) (map[string]any, bool) {
	v, ok := lookup(obj, keys)
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

// entries guards some entries of the map under path, such as labels or
// annotations: the keys listed, the keys that start with one of the prefixes,
// and with fromBase every key the generated map has.
type entries struct {
	path     []string
	keys     []string
	prefixes []string
	fromBase bool
}

func (g entries) named(key string) bool {
	if slices.Contains(g.keys, key) {
		return true
	}
	for _, p := range g.prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func (g entries) restore(base, merged map[string]any, at location, report reporter) {
	bm, _ := lookupMap(base, g.path)
	mm, _ := lookupMap(merged, g.path)

	var guarded []string
	for k := range bm {
		if g.fromBase || g.named(k) {
			guarded = append(guarded, k)
		}
	}
	for k := range mm {
		if _, ok := bm[k]; !ok && g.named(k) {
			guarded = append(guarded, k)
		}
	}
	sort.Strings(guarded)

	for _, k := range guarded {
		want, has := bm[k]
		got, set := mm[k]
		if has == set && reflect.DeepEqual(want, got) {
			continue
		}
		if has {
			if mm == nil {
				mm = ensure(merged, g.path)
			}
			mm[k] = deepCopy(want)
			report(at.child(g.path...).entry(k), "restored")
		} else {
			delete(mm, k)
			report(at.child(g.path...).entry(k), "removed")
		}
	}
}

func (g entries) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
		return
	case !present:
		return
	case val == nil:
		report(at.child(g.path...), "deletes protected entries")
		return
	}
	m, _ := val.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if g.named(k) {
			report(at.child(g.path...).entry(k), "is protected")
		}
	}
}

// listItems returns the items of the list under path, and the item whose
// mergeKey is id together with its index.
func listItems(obj map[string]any, path []string) []any {
	v, _ := lookup(obj, path)
	list, _ := v.([]any)
	return list
}

func itemID(item any, mergeKey string) (string, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	id, ok := m[mergeKey].(string)
	return id, ok
}

func findItem(list []any, mergeKey, id string) (map[string]any, int) {
	for i, item := range list {
		if got, ok := itemID(item, mergeKey); ok && got == id {
			m, _ := item.(map[string]any)
			return m, i
		}
	}
	return nil, -1
}

// items guards whole items of the list under path, identified by mergeKey:
// the ids listed and, with fromBase, every item the generated list has.
type items struct {
	path     []string
	mergeKey string
	ids      []string
	fromBase bool
}

func (g items) restore(base, merged map[string]any, at location, report reporter) {
	bl, ml := listItems(base, g.path), listItems(merged, g.path)

	var guarded []string
	for _, it := range bl {
		if id, ok := itemID(it, g.mergeKey); ok && (g.fromBase || slices.Contains(g.ids, id)) {
			guarded = append(guarded, id)
		}
	}
	for _, it := range ml {
		if id, ok := itemID(it, g.mergeKey); ok && slices.Contains(g.ids, id) && !slices.Contains(guarded, id) {
			guarded = append(guarded, id)
		}
	}

	changed := false
	for _, id := range guarded {
		want, _ := findItem(bl, g.mergeKey, id)
		got, idx := findItem(ml, g.mergeKey, id)
		switch {
		case want != nil && got == nil:
			ml = append(ml, deepCopy(want))
			report(at.child(g.path...).item(g.mergeKey, id), "restored")
		case want == nil && got != nil:
			ml = slices.Delete(ml, idx, idx+1)
			report(at.child(g.path...).item(g.mergeKey, id), "removed")
		case !reflect.DeepEqual(want, got):
			ml[idx] = deepCopy(want)
			report(at.child(g.path...).item(g.mergeKey, id), "restored")
		default:
			continue
		}
		changed = true
	}
	if changed {
		ensure(merged, g.path[:len(g.path)-1])[g.path[len(g.path)-1]] = ml
	}
}

func (g items) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
		return
	case !present:
		return
	case val == nil:
		report(at.child(g.path...), "deletes protected items")
		return
	}
	list, _ := val.([]any)
	for _, it := range list {
		if id, ok := itemID(it, g.mergeKey); ok && slices.Contains(g.ids, id) {
			report(at.child(g.path...).item(g.mergeKey, id), "is protected")
		}
	}
}

// mounts guards a container's volume mounts, identified by mountPath: the
// paths listed and every path the generated container mounts, and anything
// mounted under one of them, which would hide what the operator puts there.
// Paths compare cleaned, so /mnt/opt and /mnt/opt/ are the same mount.
type mounts struct {
	path  []string
	paths []string
}

const mountKey = "mountPath"

// reserved returns the protected path that p is, or lies under, if any.
func (g mounts) reserved(p string, generated []any) (string, bool) {
	p = path.Clean(p)
	protected := slices.Clone(g.paths)
	for _, it := range generated {
		if id, ok := itemID(it, mountKey); ok {
			protected = append(protected, id)
		}
	}
	for _, q := range protected {
		q = path.Clean(q)
		if p == q || strings.HasPrefix(p, q+"/") {
			return q, true
		}
	}
	return "", false
}

func (g mounts) restore(base, merged map[string]any, at location, report reporter) {
	bl, ml := listItems(base, g.path), listItems(merged, g.path)
	here := at.child(g.path...)

	out, changed := make([]any, 0, len(ml)), false
	for _, it := range ml {
		id, _ := itemID(it, mountKey)
		if want, _ := findItem(bl, mountKey, id); want != nil {
			if !reflect.DeepEqual(want, it) {
				it, changed = deepCopy(want), true
				report(here.item(mountKey, id), "restored")
			}
			out = append(out, it)
			continue
		}
		if _, ok := g.reserved(id, bl); ok {
			changed = true
			report(here.item(mountKey, id), "removed")
			continue
		}
		out = append(out, it)
	}
	for _, it := range bl {
		id, _ := itemID(it, mountKey)
		if got, _ := findItem(out, mountKey, id); got == nil {
			out, changed = append(out, deepCopy(it)), true
			report(here.item(mountKey, id), "restored")
		}
	}
	if changed {
		ensure(merged, g.path[:len(g.path)-1])[g.path[len(g.path)-1]] = out
	}
}

func (g mounts) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
		return
	case !present:
		return
	case val == nil:
		report(at.child(g.path...), "deletes protected items")
		return
	}
	list, _ := val.([]any)
	for _, it := range list {
		id, ok := itemID(it, mountKey)
		if !ok {
			continue
		}
		q, bad := g.reserved(id, nil)
		switch {
		case !bad:
		case q == path.Clean(id):
			report(at.child(g.path...).item(mountKey, id), "is protected")
		default:
			report(at.child(g.path...).item(mountKey, id), fmt.Sprintf("is under %s, which the operator mounts", q))
		}
	}
}

// generatedOnly keeps the list under path to the items the operator
// generates: the builders drop any other item, and admission accepts only the
// ids listed.
type generatedOnly struct {
	path     []string
	mergeKey string
	ids      []string
}

func (g generatedOnly) restore(base, merged map[string]any, at location, report reporter) {
	bl, ml := listItems(base, g.path), listItems(merged, g.path)
	kept := ml[:0:0]
	for _, it := range ml {
		id, _ := itemID(it, g.mergeKey)
		if want, _ := findItem(bl, g.mergeKey, id); want == nil {
			report(at.child(g.path...).item(g.mergeKey, id), "removed, the operator does not run it")
			continue
		}
		kept = append(kept, it)
	}
	if len(kept) != len(ml) {
		ensure(merged, g.path[:len(g.path)-1])[g.path[len(g.path)-1]] = kept
	}
}

func (g generatedOnly) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
		return
	case !present:
		return
	case val == nil:
		report(at.child(g.path...), "deletes protected items")
		return
	}
	list, _ := val.([]any)
	for _, it := range list {
		id, ok := itemID(it, g.mergeKey)
		switch {
		case !ok:
			report(at.child(g.path...), fmt.Sprintf("has an item without %s", g.mergeKey))
		case !slices.Contains(g.ids, id):
			report(at.child(g.path...).item(g.mergeKey, id), "is not generated by the operator")
		}
	}
}

// within applies guards inside the item of the list under path whose mergeKey
// is id. A generated item missing from the merged list is put back whole.
type within struct {
	path     []string
	mergeKey string
	id       string
	guards   []guard
}

func (g within) restore(base, merged map[string]any, at location, report reporter) {
	want, _ := findItem(listItems(base, g.path), g.mergeKey, g.id)
	if want == nil {
		return
	}
	here := at.child(g.path...).item(g.mergeKey, g.id)
	ml := listItems(merged, g.path)
	got, _ := findItem(ml, g.mergeKey, g.id)
	if got == nil {
		ensure(merged, g.path[:len(g.path)-1])[g.path[len(g.path)-1]] = append(ml, deepCopy(want))
		report(here, "restored")
		return
	}
	for _, sub := range g.guards {
		sub.restore(want, got, here, report)
	}
}

func (g within) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	switch {
	case nullAt != nil:
		report(nullAt, "deletes protected fields")
		return
	case !present:
		return
	case val == nil:
		report(at.child(g.path...), "deletes protected items")
		return
	}
	list, _ := val.([]any)
	if got, _ := findItem(list, g.mergeKey, g.id); got != nil {
		here := at.child(g.path...).item(g.mergeKey, g.id)
		for _, sub := range g.guards {
			sub.check(got, here, report)
		}
	}
}

// flags guards a list of command line arguments against flags the operator
// sets itself. If one of them shows up, the whole list goes back to what was
// generated: dropping only the flag would leave its value behind.
type flags struct {
	path  []string
	names []string
}

func (g flags) reserved(arg string) (string, bool) {
	name := strings.TrimLeft(arg, "-")
	if name == arg {
		return "", false
	}
	name, _, _ = strings.Cut(name, "=")
	return name, slices.Contains(g.names, name)
}

func (g flags) restore(base, merged map[string]any, at location, report reporter) {
	got, _ := lookup(merged, g.path)
	list, _ := got.([]any)
	for _, arg := range list {
		if s, ok := arg.(string); ok {
			if _, bad := g.reserved(s); bad {
				fixed(g.path).restore(base, merged, at, report)
				return
			}
		}
	}
}

func (g flags) check(patch map[string]any, at location, report reporter) {
	val, present, nullAt := walk(patch, g.path, at)
	if nullAt != nil || !present {
		return
	}
	list, _ := val.([]any)
	for _, arg := range list {
		if s, ok := arg.(string); ok {
			if name, bad := g.reserved(s); bad {
				report(at.child(g.path...), fmt.Sprintf("sets --%s, which the operator sets itself", name))
			}
		}
	}
}

// ordered keeps the generated items of the list under path in their generated
// order, followed by the items a patch added. A strategic merge puts the
// patch's items first: that would reorder the containers, and put the user's
// environment variables before the operator's, where a $(VAR) reference to one
// of the operator's could not resolve.
type ordered struct {
	path     []string
	mergeKey string
}

func (g ordered) restore(base, merged map[string]any, _ location, _ reporter) {
	bl, ml := listItems(base, g.path), listItems(merged, g.path)
	if len(ml) < 2 {
		return
	}
	out := make([]any, 0, len(ml))
	used := make([]bool, len(ml))
	for _, it := range bl {
		id, ok := itemID(it, g.mergeKey)
		if !ok {
			continue
		}
		for i, m := range ml {
			if mid, ok := itemID(m, g.mergeKey); ok && !used[i] && mid == id {
				out, used[i] = append(out, m), true
				break
			}
		}
	}
	for i, m := range ml {
		if !used[i] {
			out = append(out, m)
		}
	}
	ensure(merged, g.path[:len(g.path)-1])[g.path[len(g.path)-1]] = out
}

func (ordered) check(map[string]any, location, reporter) {}

// probeHandlers are the fields of a probe that say what it runs.
var probeHandlers = []string{"exec", "httpGet", "tcpSocket", "grpc"}

// probe guards a container probe. When the operator sets the probe, its
// handler stays as generated and only its timing may change; a probe the
// operator does not set stays unset, since a probe needs a handler.
type probe struct {
	key          string
	operatorSets bool
}

func (g probe) restore(base, merged map[string]any, at location, report reporter) {
	_, generated := base[g.key]
	_, kept := merged[g.key].(map[string]any)
	if !generated || !kept {
		fixed{g.key}.restore(base, merged, at, report)
		return
	}
	for _, h := range probeHandlers {
		fixed{g.key, h}.restore(base, merged, at, report)
	}
}

func (g probe) check(patch map[string]any, at location, report reporter) {
	if !g.operatorSets {
		fixed{g.key}.check(patch, at, report)
		return
	}
	if v, ok := patch[g.key]; ok && v == nil {
		report(at.child(g.key), "deletes protected fields")
		return
	}
	for _, h := range probeHandlers {
		fixed{g.key, h}.check(patch, at, report)
	}
}

// exclusive guards keys under path of which at most one may be set, such as a
// PodDisruptionBudget's minAvailable and maxUnavailable; the API server
// rejects an object with both. The operator sets generated, and a patch that
// sets one of the others replaces it. That needs no null to delete generated,
// which client-side kubectl apply and merge patches drop from spec.overwrites.
// A patch that also changes generated sets both, and keeps only generated.
type exclusive struct {
	path      []string
	generated string
	others    []string
}

func (g exclusive) restore(base, merged map[string]any, at location, report reporter) {
	m, ok := lookupMap(merged, g.path)
	if !ok {
		return
	}
	got, set := m[g.generated]
	if !set {
		return
	}
	want, _ := lookup(base, prefixed(g.path, g.generated))
	for _, k := range g.others {
		if _, other := m[k]; !other {
			continue
		}
		if reflect.DeepEqual(want, got) {
			delete(m, g.generated)
			continue
		}
		delete(m, k)
		report(at.child(g.path...).child(k), fmt.Sprintf("removed, it cannot be set together with %s", g.generated))
	}
}

func (g exclusive) check(patch map[string]any, at location, report reporter) {
	val, present, _ := walk(patch, g.path, at)
	m, ok := val.(map[string]any)
	if !present || !ok {
		return
	}
	if v, set := m[g.generated]; !set || v == nil {
		return
	}
	for _, k := range g.others {
		if v, set := m[k]; set && v != nil {
			report(at.child(g.path...).child(k), fmt.Sprintf("cannot be set together with %s", g.generated))
		}
	}
}
