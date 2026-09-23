package jsongen_test

import (
	"bytes"
	"errors"
	"testing"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/wire"
)

// isStrong is the pipeline's precomputed method.
func isStrong(p *ir.Package) *ir.ExportFn { return p.Types[0].(*ir.Record).Methods[0] }

// An IR stage E or the checker refuses is an error, never a file nor a finding (decision 126).
func TestRefusals(t *testing.T) {
	cases := []struct {
		name  string
		p     func() *ir.Package
		setup func(p *ir.Package) *ir.Emit
		want  error
	}{
		{"a cpp emit", pipeline, func(p *ir.Package) *ir.Emit { return p.Emits[1] }, jsongen.ErrEmit},
		{"no emit", pipeline, func(*ir.Package) *ir.Emit { return nil }, jsongen.ErrEmit},
		// WIRE.md §8.1, E8150: file mode writes exactly one value, to a `.json` name (case-sensitive).
		{"file mode, seven values", teamboard, func(p *ir.Package) *ir.Emit {
			e := p.Emits[2]
			e.FileName = "taxonomy.json"
			return e
		}, jsongen.ErrFileMode},
		{"file mode, .JSON", pipeline, func(p *ir.Package) *ir.Emit { p.Emits[0].FileName = "potions.JSON"; return p.Emits[0] }, jsongen.ErrFileMode},
		{"file mode, a path", pipeline, func(p *ir.Package) *ir.Emit { p.Emits[0].FileName = "a/potions.json"; return p.Emits[0] }, jsongen.ErrFileMode},
		// CODEGEN.md §2.1, E8009: values name public lets, each once.
		{"unknown value", teamboard, func(*ir.Package) *ir.Emit { return jsonEmit("data", "columns") }, jsongen.ErrValues},
		{"value twice", teamboard, func(*ir.Package) *ir.Emit { return jsonEmit("data", "deck", "deck") }, jsongen.ErrValues},
		{"two public values with one name", teamboard, func(p *ir.Package) *ir.Emit {
			p.Values[1].Name = p.Values[0].Name
			return p.Emits[2]
		}, jsongen.ErrValues},
		// WIRE.md §8.1, E8153: data-mode and @reload values are written, as <value>.json.
		{"@reload value renamed", pipeline, func(p *ir.Package) *ir.Emit { p.Emits[0].FileName = "items.json"; return p.Emits[0] }, jsongen.ErrDataMode},
		{"@reload value not written", teamboard, func(p *ir.Package) *ir.Emit {
			p.Values[5].Reload = true
			return jsonEmit("data", "deck")
		}, jsongen.ErrDataMode},
		{"data-mode go emit reads more", teamboard, func(p *ir.Package) *ir.Emit {
			p.Emits[0].Mode = ir.ModeData
			return jsonEmit("data", "statuses")
		}, jsongen.ErrDataMode},
		{"data-mode ts emit of one unwritten value", teamboard, func(p *ir.Package) *ir.Emit {
			p.Emits[1].Mode, p.Emits[1].Values = ir.ModeData, []string{"areas"}
			return jsonEmit("data", "statuses")
		}, jsongen.ErrDataMode},
		// FINGERPRINT.md §2: the loaders' compiled-in identifier is the file's.
		{"stale Value.Schema", pipeline, func(p *ir.Package) *ir.Emit { p.Values[0].Schema = "pipeline.Potion@00000000"; return p.Emits[0] }, jsongen.ErrSchema},
		// WIRE.md §5.11: every stored fn has its data, every receiver its instance.
		{"receiver without instance", pipeline, func(p *ir.Package) *ir.Emit {
			m := isStrong(p)
			m.Instances = m.Instances[:1]
			return p.Emits[0]
		}, jsongen.ErrFn},
		{"instance twice", pipeline, func(p *ir.Package) *ir.Emit {
			m := isStrong(p)
			m.Instances = append(m.Instances, m.Instances[0])
			return p.Emits[0]
		}, jsongen.ErrFn},
		{"instance without result", pipeline, func(p *ir.Package) *ir.Emit { isStrong(p).Instances[1].Result = nil; return p.Emits[0] }, jsongen.ErrFn},
		{"lookup fn without table", teamboard, func(p *ir.Package) *ir.Emit { p.Fns[0].Table = nil; return p.Emits[2] }, jsongen.ErrFn},
		{"precomputed fn without value", teamboard, func(p *ir.Package) *ir.Emit {
			p.Fns = append(p.Fns, &ir.ExportFn{Name: "count", Kind: ir.FnPrecomputed, Result: tInt.ir})
			return p.Emits[2]
		}, jsongen.ErrFn},
		// WIRE.md §5.9, E8151: stage E refuses a type with no wire form first.
		{"Range value", teamboard, func(p *ir.Package) *ir.Emit { p.Values[3].Type = ir.TypeRef{Kind: types.Range}; return p.Emits[2] }, ir.ErrFingerprint},
		{"value unlike its type", teamboard, func(p *ir.Package) *ir.Emit { p.Values[1].V = str("open"); return p.Emits[2] }, wire.ErrShape},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.p()
			files, err := jsongen.Generate(p, c.setup(p))
			if !errors.Is(err, c.want) || files != nil {
				t.Errorf("got %d files, %v; want %v", len(files), err, c.want)
			}
		})
	}
	if _, err := jsongen.Generate(nil, jsonEmit("data")); !errors.Is(err, jsongen.ErrEmit) {
		t.Errorf("nil package: %v", err)
	}
}

// E8152 is build's, over every output of the build: both colliding files are returned (decision 126).
func TestCaseCollisionIsLeftToBuild(t *testing.T) {
	p := teamboard()
	p.Fns = nil
	upper := *p.Values[2]
	upper.Name = "AssigneeMinRole"
	p.Values = append(p.Values, &upper)
	files := generate(t, p, jsonEmit("data", "assigneeMinRole", "AssigneeMinRole"))
	if len(files) != 2 || files[0].Path != "assigneeMinRole.json" || files[1].Path != "AssigneeMinRole.json" {
		t.Errorf("files: %+v", files)
	}
}

// WIRE.md §5.11: translated fns are never data: no `$fns`, and a schema without them.
func TestTranslatedFnsAreNotData(t *testing.T) {
	p := teamboard()
	p.Fns = []*ir.ExportFn{{Name: "clampHidden", Kind: ir.FnTranslated, Params: []*ir.Param{{Name: "n", Type: tInt.ir}}, Result: tInt.ir}}
	files := generate(t, p, p.Emits[2])
	want, err := ir.Schema(tb, "intents", &p.Values[0].Type, nil)
	if err != nil || bytes.Contains(files[0].Content, []byte(`"$fns"`)) || !bytes.Contains(files[0].Content, []byte(want)) {
		t.Errorf("intents.json (%v):\n%s", err, files[0].Content)
	}
}
