package build

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
)

// DECISIONS 196, meta/decisions/log-2026-09-24.md "ir export fns round 2": a translated fn stage E
// left untranslated (ExportFn.Err) is an internal error of the run, wherever the fn sits.
func TestUntranslated(t *testing.T) {
	failed := func(name string) *ir.ExportFn {
		return &ir.ExportFn{Name: name, Err: fmt.Errorf("%w: %s", ir.ErrInternal, name)}
	}
	for _, c := range []struct {
		name string
		pkg  *ir.Package
		want int // the internal errors joined
	}{
		{"none", &ir.Package{Fns: []*ir.ExportFn{{Name: "f"}}}, 0},
		{"package fn", &ir.Package{Fns: []*ir.ExportFn{{Name: "f"}, failed("g")}}, 1},
		{"record method", &ir.Package{Types: []ir.Type{&ir.Record{Methods: []*ir.ExportFn{failed("m")}}}}, 1},
		{"case method", &ir.Package{Types: []ir.Type{&ir.Variant{Cases: []*ir.Case{{Methods: []*ir.ExportFn{failed("m")}}}}}}, 1},
		{"all", &ir.Package{
			Types: []ir.Type{&ir.Enum{}, &ir.Record{Methods: []*ir.ExportFn{failed("m")}}},
			Fns:   []*ir.ExportFn{failed("g")},
		}, 2},
	} {
		err := untranslated([]*ir.Package{c.pkg})
		if c.want == 0 {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		var joined interface{ Unwrap() []error }
		if !errors.Is(err, ErrInternal) || !errors.Is(err, ir.ErrInternal) || !errors.As(err, &joined) || len(joined.Unwrap()) != c.want {
			t.Errorf("%s: %v, want %d internal errors", c.name, err, c.want)
		}
	}
}

// ADR-0003: each of the evaluator's limits is conform's limit of the same meaning.
func TestLimits(t *testing.T) {
	want := []struct {
		from eval.Limit
		to   conform.Limit
	}{{eval.NoLimit, conform.NoLimit}, {eval.StepLimit, conform.StepLimit}, {eval.DepthLimit, conform.DepthLimit}}
	if len(limits) != len(want) {
		t.Fatalf("%d limits mapped, want %d", len(limits), len(want))
	}
	for _, w := range want {
		if limits[w.from] != w.to {
			t.Errorf("eval limit %d maps to %d, want %d", w.from, limits[w.from], w.to)
		}
	}
}
