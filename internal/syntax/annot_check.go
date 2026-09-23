package syntax

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// boundArg is an annotation argument matched with its entry in the catalogue.
type boundArg struct {
	spec *argSpec
	arg  *AnnotationArg
}

// checkAnnotations checks the annotations at one site against the catalogue: names (E1104),
// positions (E1118), values (E1119), duplicates (E1120) and argument order (E1121). It reports
// whether @codes is among them.
func (p *parser) checkAnnotations(anns []*Annotation, site annSite) bool {
	seen := map[string]bool{}
	var jsonCodes *Annotation
	for _, a := range anns {
		name := a.Name.Name
		if seen[name] {
			diag.E1120.At(p.nodeSpan(a), name).Report(p.bag)
		}
		seen[name] = true
		if bound := p.checkAnnotation(a, site); name == annJSON && hasFlag(bound, annCodes) {
			jsonCodes = a
		}
	}
	if jsonCodes != nil && !seen[annCodes] && site == siteEnumHeader {
		diag.E1119.AtCodes(p.nodeSpan(jsonCodes)).Report(p.bag)
	}
	return seen[annCodes]
}

// checkAnnotation checks one annotation and returns its arguments matched to the catalogue.
func (p *parser) checkAnnotation(a *Annotation, site annSite) []boundArg {
	name := a.Name.Name
	spec := annCatalog[name]
	if spec == nil {
		diag.E1104.AtAnnotation(p.nodeSpan(a.Name), name).Report(p.bag)
		return nil
	}
	bound := p.bindArgs(a, spec)
	if !placed(spec, bound, site, len(a.Args) == 0) {
		diag.E1118.At(p.nodeSpan(a), name, siteKind(site)).Report(p.bag)
	}
	for _, b := range bound {
		p.checkValue(name, b)
	}
	p.checkCombos(a, spec, bound)
	return bound
}

// placed reports an annotation allowed at site: bare where its bare form or one of its
// arguments may stand, with arguments where each of them may.
func placed(spec *annSpec, bound []boundArg, site annSite, bare bool) bool {
	if bare {
		allowed := spec.bare
		for _, a := range spec.args {
			allowed |= a.sites
		}
		return allowed&site != 0
	}
	for _, b := range bound {
		if b.spec.sites&site == 0 {
			return false
		}
	}
	return true
}

// bindArgs matches each argument with the catalogue: unknown ones are E1104, a positional one
// after a named one and one given twice E1121.
func (p *parser) bindArgs(a *Annotation, spec *annSpec) []boundArg {
	var out []boundArg
	named := false
	used := map[*argSpec]bool{}
	for _, arg := range a.Args {
		if arg.Name != nil {
			named = true
		} else if named {
			diag.E1121.AtOrder(p.nodeSpan(arg)).Report(p.bag)
		}
		s := findArg(spec, arg)
		switch {
		case s == nil:
			diag.E1104.AtArgument(p.nodeSpan(arg), a.Name.Name, p.argLabel(arg)).Report(p.bag)
		case used[s]:
			diag.E1121.AtTwice(p.nodeSpan(arg), s.name).Report(p.bag)
		default:
			used[s] = true
			out = append(out, boundArg{spec: s, arg: arg})
		}
	}
	return out
}

// findArg is the catalogue entry of arg: by name, as a flag, or as the positional argument.
func findArg(spec *annSpec, arg *AnnotationArg) *argSpec {
	sym := symbolOf(arg.Value)
	for i := range spec.args {
		s := &spec.args[i]
		switch {
		case arg.Name != nil:
			if s.named && s.name == arg.Name.Name {
				return s
			}
		case s.kind == valFlag:
			if sym == s.name {
				return s
			}
		case !s.named && (sym == "" || s.kind == valSymbol || s.kind == valStudio):
			return s
		}
	}
	return nil
}

// argLabel names an argument in E1104: its name, or its value as written.
func (p *parser) argLabel(arg *AnnotationArg) string {
	if arg.Name != nil {
		return arg.Name.Name
	}
	return p.found(arg.Value.First())
}

// checkValue reports a value of the wrong kind or outside its closed set (E1119).
func (p *parser) checkValue(name string, b boundArg) {
	v, sp := b.arg.Value, p.nodeSpan(b.arg)
	switch s := b.spec; s.kind {
	case valSymbol, valStudio:
		if sym := symbolOf(v); sym == "" || (s.kind == valSymbol && !slices.Contains(s.values, sym)) {
			diag.E1119.AtValue(sp, name, s.name, s.values).Report(p.bag)
		}
	case valFlag:
	default:
		if !valueOK[s.kind](v) {
			diag.E1119.AtKind(sp, name, s.name, expectedKind[s.kind]).Report(p.bag)
		}
	}
}

// checkCombos reports exclusive arguments, an argument without the one it needs and a missing
// required argument (E1119).
func (p *parser) checkCombos(a *Annotation, spec *annSpec, bound []boundArg) {
	name := a.Name.Name
	has := map[string]bool{}
	for i, b := range bound {
		has[b.spec.name] = true
		for _, prev := range bound[:i] {
			if prev.spec.groups&b.spec.groups != 0 {
				diag.E1119.AtExclusive(p.nodeSpan(b.arg), name, b.spec.name, prev.spec.name).Report(p.bag)
			}
		}
	}
	if need, ok := annNeeds[name]; ok && has[need.arg] && !has[need.needs] {
		diag.E1119.AtNeeds(p.nodeSpan(a), name, need.arg, need.needs).Report(p.bag)
	}
	for _, s := range spec.args {
		if s.required && !has[s.name] {
			diag.E1119.AtMissing(p.nodeSpan(a), name, s.name).Report(p.bag)
		}
	}
}

func hasFlag(bound []boundArg, flag string) bool {
	for _, b := range bound {
		if b.spec.kind == valFlag && b.spec.name == flag {
			return true
		}
	}
	return false
}

// symbolOf is the word of a one-word symbol value, or "".
func symbolOf(v AnnValue) string {
	if q, ok := v.(*QualifiedName); ok && len(q.Parts) == 1 {
		return q.Parts[0].Name
	}
	return ""
}

// siteKind is the position word of site in E1118 (GRAMMAR.md §8.1).
func siteKind(site annSite) diag.Kind {
	for _, sk := range siteKinds {
		if site&sk.sites != 0 {
			return sk.kind
		}
	}
	return diag.KindTopLevelDeclaration
}
