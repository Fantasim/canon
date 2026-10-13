package jsongen_test

import (
	"errors"
	"slices"
	"testing"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
)

// layoutErrs are the refusals of an emit's layout, which Paths makes as Generate does; the others
// are refusals of a file's content, which Paths never encodes.
var layoutErrs = []error{jsongen.ErrEmit, jsongen.ErrValues, jsongen.ErrFileMode, jsongen.ErrDataMode}

// DECISIONS 355, WIRE.md §8.1 (decision 127): Paths names Generate's files, in its order, in either mode.
func TestPathsAreGeneratesFiles(t *testing.T) {
	cases := []struct {
		name string
		p    func() *ir.Package
		emit func(p *ir.Package) *ir.Emit
	}{
		{"directory mode, every public value", teamboard, func(p *ir.Package) *ir.Emit { return p.Emits[2] }},
		{"directory mode, values listed", teamboard, func(*ir.Package) *ir.Emit { return jsonEmit("data", "intents", "deck") }},
		{"file mode, an @reload value read by data-mode emits", pipeline, func(p *ir.Package) *ir.Emit { return p.Emits[0] }},
		{"directory mode, read by a data-mode go emit", teamboard, func(p *ir.Package) *ir.Emit {
			p.Emits[0].Mode, p.Emits[0].Values = ir.ModeData, []string{"statuses"}
			return p.Emits[2]
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := c.p()
			e := c.emit(p)
			var want []string
			for _, f := range generate(t, p, e) {
				want = append(want, f.Path)
			}
			got, err := jsongen.Paths(p, e)
			if err != nil || !slices.Equal(got, want) {
				t.Errorf("Paths: %v, %v; Generate wrote %v", got, err, want)
			}
		})
	}
}

// DECISIONS 355, decision 126: Paths refuses only the layouts Generate refuses, with its errors.
func TestPathsRefuseTheLayoutOnly(t *testing.T) {
	for _, c := range refusals() {
		t.Run(c.name, func(t *testing.T) {
			p := c.p()
			paths, err := jsongen.Paths(p, c.setup(p))
			layout := slices.ContainsFunc(layoutErrs, func(e error) bool { return errors.Is(c.want, e) })
			switch {
			case layout && (!errors.Is(err, c.want) || paths != nil):
				t.Errorf("got %v, %v; want %v", paths, err, c.want)
			case !layout && (err != nil || len(paths) == 0):
				t.Errorf("got %v, %v; want the files named", paths, err)
			}
		})
	}
	if _, err := jsongen.Paths(nil, jsonEmit("data")); !errors.Is(err, jsongen.ErrEmit) {
		t.Errorf("nil package: %v", err)
	}
}
