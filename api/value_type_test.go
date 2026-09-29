package canon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// modelTypes is each value's and field's type expression as a view model writes it (VIEWMODEL.md §12.3).
type modelTypes struct {
	Types map[string]struct {
		Fields []modelField `json:"fields"`
		Cases  []struct {
			Name   string       `json:"name"`
			Fields []modelField `json:"fields"`
		} `json:"cases"`
	} `json:"types"`
	Values map[string]struct {
		Type json.RawMessage `json:"type"`
	} `json:"values"`
}

type modelField struct {
	Name string          `json:"name"`
	Type json.RawMessage `json:"type"`
}

// typeHead is what the walk reads of a type expression: its kind, the type it names, what it holds.
type typeHead struct {
	Kind  string          `json:"kind"`
	Ref   string          `json:"ref"`
	Of    json.RawMessage `json:"of"`
	Value json.RawMessage `json:"value"`
}

// typeWalk compares, down a value's parts, each part's TypeInfo.VM with the model's: a field's
// with its field definition, an element's with its list's or table's `of`, a map value's with its
// map's `value`. It counts each comparison by site and kind.
type typeWalk struct {
	t      *testing.T
	p      *canon.Project
	models map[string]*modelTypes
	seen   map[string]int // "<site>:<kind>", and "sibling" for a per-instance ref (J12)
}

func newTypeWalk(t *testing.T, p *canon.Project) *typeWalk {
	return &typeWalk{t: t, p: p, models: map[string]*modelTypes{}, seen: map[string]int{}}
}

// Comparison sites of a typeWalk.
const (
	siteRoot   = "root"
	siteField  = "field"
	siteElem   = "elem"
	siteMapVal = "mapval"
	seenSib    = "sibling"
)

// exampleKinds are the sites and kinds the examples must reach: every §12.3 row they use.
var exampleKinds = []string{
	"root:table", "root:list", "root:record", "root:enum", "root:ref",
	"field:bool", "field:int", "field:float", "field:string", "field:duration", "field:enum",
	"field:record", "field:variant", "field:list", "field:map", "field:optional", "field:ref",
	"field:asset", "field:dependent", "elem:record", "elem:ref", "elem:dependent", "mapval:int",
	"mapval:record",
}

// API.md §5.2: TypeInfo.VM is the model's type expression, for every example's public lets and parts.
func TestValueTypeVM(t *testing.T) {
	p, _ := openViewExamples(t)
	infos, err := p.Packages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := newTypeWalk(t, p)
	for _, info := range infos {
		w.roots(info.Name)
	}
	for _, k := range exampleKinds {
		if w.seen[k] == 0 {
			t.Errorf("no %s compared: %v", k, w.seen)
		}
	}
}

// roots compares every public let of pkg's model and walks its parts.
func (w *typeWalk) roots(pkg string) {
	m := w.model(pkg)
	for _, root := range slices.Sorted(maps.Keys(m.Values)) {
		v, err := w.p.Value(context.Background(), root)
		if err != nil {
			w.t.Fatalf("Value(%s): %v", root, err)
		}
		w.same(siteRoot, root, v.Type.VM, m.Values[root].Type)
		w.walk(v)
	}
}

// model is pkg's view model as the test reads it.
func (w *typeWalk) model(pkg string) *modelTypes {
	if mt, ok := w.models[pkg]; ok {
		return mt
	}
	m, err := w.p.ViewModel(context.Background(), pkg)
	if err != nil {
		w.t.Fatalf("ViewModel(%s): %v", pkg, err)
	}
	mt := &modelTypes{}
	if err := json.Unmarshal(m.JSON(), mt); err != nil {
		w.t.Fatal(err)
	}
	w.models[pkg] = mt
	return mt
}

// walk compares v's parts with the model, then walks each of them.
func (w *typeWalk) walk(v *canon.Value) {
	var head typeHead
	_ = json.Unmarshal(v.Type.VM, &head)
	fields := w.fields(v, head)
	for _, c := range v.Children() {
		seg := strings.TrimPrefix(c.Path, v.Path)
		switch {
		case strings.HasPrefix(seg, "."):
			if i := slices.IndexFunc(fields, func(f modelField) bool { return "."+f.Name == seg }); i >= 0 {
				w.same(siteField, c.Path, c.Type.VM, fields[i].Type)
			}
		case (head.Kind == "list" || head.Kind == "table") && head.Of != nil:
			w.same(siteElem, c.Path, c.Type.VM, head.Of)
		case head.Kind == "map":
			w.same(siteMapVal, c.Path, c.Type.VM, head.Value)
		}
		w.walk(c)
	}
}

// fields are the model's fields of v's record or current case; nil for any other value.
func (w *typeWalk) fields(v *canon.Value, head typeHead) []modelField {
	if head.Ref == "" || (head.Kind != "record" && head.Kind != "variant") {
		return nil
	}
	def := w.model(head.Ref[:strings.LastIndexByte(head.Ref, '.')]).Types[head.Ref]
	c, isVariant := v.Case()
	if !isVariant {
		return def.Fields
	}
	for _, cs := range def.Cases {
		if cs.Name == c {
			return cs.Fields
		}
	}
	return nil
}

// same reports got differing from the model's compacted bytes of want, and counts the comparison.
func (w *typeWalk) same(site, path string, got, want json.RawMessage) {
	w.t.Helper()
	var compact bytes.Buffer
	if err := json.Compact(&compact, want); err != nil {
		w.t.Fatalf("%s: model type %q: %v", path, want, err)
	}
	var head typeHead
	_ = json.Unmarshal(want, &head)
	w.seen[site+":"+head.Kind]++
	if bytes.Contains(want, []byte(`"sibling"`)) {
		w.seen[seenSib]++
	}
	if !bytes.Equal(got, compact.Bytes()) {
		w.t.Errorf("%s: TypeInfo.VM = %s, want %s", path, got, compact.Bytes())
	}
}

// API.md S7: several goroutines read a Value's parts and their TypeInfo.VM at once (run with -race).
func TestValueTypeVMConcurrent(t *testing.T) {
	p, _ := openViewExamples(t)
	v, err := p.Value(context.Background(), "resource.farm:farm.global")
	if err != nil {
		t.Fatal(err)
	}
	const readers = 4
	got := make(chan string, readers)
	for range readers {
		go func() {
			var b strings.Builder
			for _, c := range v.Children() {
				b.Write(c.Type.VM)
			}
			got <- b.String()
		}()
	}
	first := <-got
	for range readers - 1 {
		if again := <-got; again != first || first == "" {
			t.Errorf("concurrent reads differ: %q vs %q", again, first)
		}
	}
}
