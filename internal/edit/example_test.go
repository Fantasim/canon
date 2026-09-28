package edit_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// A key is a key, never a position, whatever its type; [#n] is a position (API.md P1, P4).
func ExampleParse() {
	p, err := edit.Parse(`resource.farm:farm.modelTypes[3].styles["daily"][#0]`)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Package, p.Root, len(p.Segs), p.Segs[1].Key.Int, p.Segs[3].Key.Text, p.Segs[4].Pos)
	fmt.Println(p)
	// Output: resource.farm farm 5 3 daily 0
	// resource.farm:farm.modelTypes[3].styles["daily"][#0]
}

// A resolved path keeps, per segment, the container it was read in and the value there.
func ExampleResolved() {
	p, err := edit.Parse("p:levels[1]")
	if err != nil {
		fmt.Println(err)
		return
	}
	levels := &types.ListType{Elem: types.IntType}
	second := &value.Int{V: 20, T: types.IntType}
	r := edit.Resolved{Canonical: p.String(), Steps: []edit.Step{{Seg: p.Segs[0], Container: levels, Value: second}}, Target: second}
	for _, s := range r.Steps {
		fmt.Println(r.Canonical, s.Seg.Key.Int, s.Container, s.Value.CanonText(), r.Target == s.Value)
	}
	// Output: p:levels[1] 1 [Int] 20 true
}

// Resolve reads a keyed list by key (P1), a map by an enum's wire value (P2), in canonical form (P8).
func ExampleResolve() {
	p, err := build.Open(law, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		fmt.Println(err)
		return
	}
	s := edit.NewSnapshot(a)
	for _, in := range []string{"a:items[#1].n", `a:byColor["GREEN"]`} {
		path, _ := edit.Parse(in)
		r, err := edit.Resolve(s, path)
		if err != nil {
			fmt.Println(in, err)
			continue
		}
		t, _ := s.Type(r)
		fmt.Println(r.Canonical, r.Target.CanonText(), t, len(r.Steps))
	}
	// Output:
	// a:items[b].n 2 Int 2
	// a:byColor[green] 2 Int 1
}

// Editable names the layer that sets a value, or the source of a computed one.
func ExampleSnapshot_Editable() {
	p, err := build.Open(law, "/law", build.Options{Layers: []string{"dev"}})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		fmt.Println(err)
		return
	}
	s := edit.NewSnapshot(a)
	for _, in := range []string{"a:base.x", "a:mixed[0]"} {
		path, _ := edit.Parse(in)
		r, _ := edit.Resolve(s, path)
		e, _ := s.Editable(r, edit.OpSet, "")
		fmt.Printf("%s: mode %d reason %d file %q origin %q layer %q\n", in, e.Mode, e.Reason, e.File, e.Origin, e.Layer)
	}
	// Output:
	// a:base.x: mode 0 reason 2 file "" origin "" layer "dev"
	// a:mixed[0]: mode 0 reason 1 file "" origin "a:shared" layer ""
}
