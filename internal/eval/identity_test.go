package eval_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

// enclosing is a record enclosing a value met walking a let's value, and its path.
type enclosing struct {
	rec  *value.Record
	path string
}

// node is one value met walking a let's value: its path, and the records enclosing it, outermost first.
type node struct {
	v     value.Value
	path  string
	outer []enclosing
}

// walkValue visits v and every value it holds, pre-order.
func walkValue(v value.Value, path string, visit func(node)) {
	walkIn(v, path, nil, visit)
}

func walkIn(v value.Value, path string, outer []enclosing, visit func(node)) {
	if v == nil {
		return
	}
	visit(node{v: v, path: path, outer: outer})
	switch x := v.(type) {
	case *value.Record:
		inner := append(slices.Clip(outer), enclosing{rec: x, path: path})
		for i, f := range verify.Fields(x.T) {
			if i < len(x.Fields) {
				walkIn(x.Fields[i], path+"."+f.Name, inner, visit)
			}
		}
	case *value.Table:
		for _, e := range x.Entries {
			walkIn(e, path+"."+e.Ident.Key.Text(), outer, visit)
		}
	case *value.List:
		for i, e := range x.Elems {
			walkIn(e, fmt.Sprintf("%s[%d]", path, i), outer, visit)
		}
	case *value.Map:
		for i, k := range x.Keys {
			walkIn(k, fmt.Sprintf("%s{%d}", path, i), outer, visit)
			walkIn(x.Vals[i], fmt.Sprintf("%s[%s]", path, k.CanonText()), outer, visit)
		}
	case *value.Pair:
		walkIn(x.A, path+".a", outer, visit)
		walkIn(x.B, path+".b", outer, visit)
	}
}

// ownerOf is the record type holding c, nil for a collection that is no record field.
func ownerOf(c *types.Collection) *types.RecordType {
	if c == nil || c.Kind != types.CollField {
		return nil
	}
	return c.Owner
}

func recordType(t types.Type) *types.RecordType {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x
	case *types.AppliedRecord:
		return x.Rec
	}
	return nil
}

// nearest is the innermost record of outer of type rt: the instance a level-1 ref or an entry of rt's collection belongs to.
func nearest(outer []enclosing, rt *types.RecordType) (enclosing, bool) {
	for _, in := range slices.Backward(outer) {
		if recordType(in.rec.T) == rt {
			return in, true
		}
	}
	return enclosing{}, false
}

// sameInstance reports owner as the record in the tree, or a copy of its instance sharing every
// field with it (a default's copy of its literal, an entry's copy into a collection).
func sameInstance(ev *eval.Evaluator, owner, rec *value.Record) bool {
	if owner == rec {
		return true
	}
	return ev.InstanceOf(owner) == ev.InstanceOf(rec) && slices.Equal(owner.Fields, rec.Fields)
}

// bound are the places, relative to the instance they are bound to, where the refs of other
// lets' values sit: a ref placed from one of them elsewhere keeps that binding.
type bound map[*value.Ref]map[string]bool

// boundIn records where v's refs sit relative to the instance they are bound to.
func (b bound) boundIn(v value.Value) {
	walkValue(v, "", func(n node) {
		x, isRef := n.v.(*value.Ref)
		rt, ok := refType(x, isRef)
		if !ok {
			return
		}
		if in, found := nearest(n.outer, ownerOf(rt.Target)); found && in.rec == x.Owner {
			if b[x] == nil {
				b[x] = map[string]bool{}
			}
			b[x][strings.TrimPrefix(n.path, in.path)] = true
		}
	})
}

// refType is the type of a bound level-1 ref.
func refType(x *value.Ref, isRef bool) (*types.RefType, bool) {
	if !isRef || x.Owner == nil {
		return nil, false
	}
	rt, ok := x.T.Base().(*types.RefType)
	return rt, ok && ownerOf(rt.Target) != nil
}

// ownerProblems lists the refs and entries of v whose owner is not the enclosing instance they belong to.
func ownerProblems(ev *eval.Evaluator, v value.Value, name string, others bound) []string {
	var out []string
	walkValue(v, name, func(n node) {
		ok := true
		switch x := n.v.(type) {
		case *value.Ref:
			ok = refOwned(ev, x, n, others)
		case *value.Record:
			if x.Ident != nil && x.Ident.Owner != nil {
				in, found := nearest(n.outer, ownerOf(x.Ident.Coll))
				ok = !found || sameInstance(ev, x.Ident.Owner, in.rec)
			}
		}
		if !ok {
			out = append(out, n.path)
		}
	})
	return out
}

// refOwned reports a bound level-1 ref equal to the entry of its key in its nearest instance, or
// bound to that instance, or placed here from another let's value, which bound it elsewhere in it.
func refOwned(ev *eval.Evaluator, x *value.Ref, n node, others bound) bool {
	rt, ok := refType(x, true)
	if !ok {
		return true
	}
	in, found := nearest(n.outer, ownerOf(rt.Target))
	if !found {
		return true
	}
	e := entryOf(in.rec, rt.Target.FieldPath[0], x.Key)
	if e != nil && value.Equal(x, e) || e == nil && sameInstance(ev, x.Owner, in.rec) {
		return true
	}
	places, placed := others[x]
	return placed && !places[strings.TrimPrefix(n.path, in.path)]
}

// entryOf is the entry of key k in rec's collection field name, nil when none.
func entryOf(rec *value.Record, name string, k value.Key) *value.Record {
	for i, f := range verify.Fields(rec.T) {
		if f.Name != name || i >= len(rec.Fields) {
			continue
		}
		var elems []value.Value
		switch c := rec.Fields[i].(type) {
		case *value.Table:
			for _, e := range c.Entries {
				elems = append(elems, e)
			}
		case *value.List:
			elems = c.Elems
		}
		for _, el := range elems {
			if e, isRec := el.(*value.Record); isRec && e.Ident != nil && e.Ident.Key == k {
				return e
			}
		}
	}
	return nil
}

// EVALUATION.md §3.4, §4.2, TYPES.md §10.2: refs and entries name their tree's instances, with or without layers.
func TestInstanceOwners(t *testing.T) {
	files, err := filepath.Glob("testdata/parity/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob("testdata/layers/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range append(files, more...) {
		a, err := txtar.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		layers := strings.Fields(string(archiveFile(a, layersFile)))
		for _, opt := range []eval.Options{{}, {Layers: layers}} {
			checkOwners(t, runBuild(t, fromArchive(t, a), opt), file)
		}
	}
	examples, err := filepath.Glob("testdata/examples/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range examples {
		a, err := txtar.ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		dirs, pkgs := archivePackages(a)
		checkOwners(t, runBuild(t, fromExamples(t, dirs...), eval.Options{}, pkgs...), file)
	}
}

// checkOwners reports each value of b holding a ref or an entry whose owner is not its enclosing instance.
func checkOwners(t *testing.T, b *build, file string) {
	t.Helper()
	for _, root := range b.order {
		others := bound{}
		for _, other := range b.order {
			if other != root {
				others.boundIn(b.values[other])
			}
		}
		if bad := ownerProblems(b.ev, b.values[root], root.Name, others); len(bad) > 0 {
			t.Errorf("%s: %s: owner not the enclosing instance at %v", file, root.Name, bad)
		}
	}
}

// identities lists, for each ref v holds, the paths of the entries of v it equals (TYPES.md §7.5).
func identities(v value.Value) string {
	var refs, entries []node
	walkValue(v, "", func(n node) {
		switch x := n.v.(type) {
		case *value.Ref:
			refs = append(refs, n)
		case *value.Record:
			if x.Ident != nil {
				entries = append(entries, n)
			}
		}
	})
	var sb strings.Builder
	for _, r := range refs {
		sb.WriteString(r.path + " =")
		for _, e := range entries {
			if value.Equal(r.v, e.v) {
				sb.WriteString(" " + e.path)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
