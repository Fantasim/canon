package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// Site is where a finding about a value is reported (EVALUATION.md §13).
type Site struct {
	Span source.Span
	prov *value.Prov
}

// SiteOf is the site of v; a value without provenance has no location.
func SiteOf(v value.Value) Site {
	p := v.Prov()
	for p != nil && p.Kind == value.ProvSpread && p.Via != nil {
		p = p.Via
	}
	if p == nil {
		return Site{}
	}
	return Site{Span: p.Span, prov: p}
}

// Report adds the value path and the site's pointer, layer and stack to b, then reports it.
func (s Site) Report(b *diag.Builder, at *Path, bag *diag.Bag) {
	s.reportAt(b, at.String(), bag)
}

// reportAt is Report at a path already written out.
func (s Site) reportAt(b *diag.Builder, path string, bag *diag.Bag) {
	b.Path(path)
	if p := s.prov; p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer).Stack(p.Stack).MoreFrames(p.MoreFrames)
	}
	b.Report(bag)
}
