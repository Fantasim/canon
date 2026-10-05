package progen_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// refFields are the field names of tg's package whose type holds a ref.
func refFields(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			if strings.Contains(text(p, f.Type), "ref ") {
				out[f.Name.Name] = true
			}
		}
	}
	return out
}

// refIdents are the bare identifiers given to ref fields in let literals.
func refIdents(tg target) []*syntax.IdentExpr {
	fields := refFields(tg)
	var out []*syntax.IdentExpr
	for _, b := range valueLiterals(tg) {
		for _, it := range b.Items {
			f, ok := it.(*syntax.FieldItem)
			if !ok || !fields[f.Name.Name] {
				continue
			}
			syntax.Inspect(f.Value, func(n syntax.Node) bool {
				if id, ok := n.(*syntax.IdentExpr); ok {
					out = append(out, id)
				}
				return true
			})
		}
	}
	return out
}

// retiredKeys are the keys of retired entries of the file's table literals.
func retiredKeys(tg target) []string {
	var out []string
	for _, it := range nodes[*syntax.EntryItem](tg) {
		if it.Mods != nil && it.Mods.Retired.Valid() {
			out = append(out, it.Key.Name)
		}
	}
	return out
}

func refToRetired(tg target) []progen.Site {
	var out []progen.Site
	for _, key := range retiredKeys(tg) {
		for _, it := range nodes[*syntax.EntryItem](tg) {
			if it.Mods != nil && it.Mods.Retired.Valid() {
				continue
			}
			syntax.Inspect(it.Value, func(n syntax.Node) bool {
				if id, ok := n.(*syntax.IdentExpr); ok && id.Name != key && isRefIdent(tg, id) {
					s, e := span(tg, id)
					out = append(out, site(replace(s, e, key)))
				}
				return true
			})
		}
	}
	return out
}

func isRefIdent(tg target, id *syntax.IdentExpr) bool {
	for _, r := range refIdents(tg) {
		if r == id {
			return true
		}
	}
	return false
}

func refToConst(tg target) []progen.Site {
	var out []progen.Site
	for _, c := range declared(tg, func(d *syntax.ConstDecl) *syntax.Ident { return d.Name }) {
		for _, r := range nodes[*syntax.RefType](tg) {
			s, e := span(tg, r.Name)
			out = append(out, site(replace(s, e, c)))
		}
	}
	return out
}

// retiredMemberUse retires a member its file's lets name once, its only use: each is E3506 (TYPES.md §8.1).
func retiredMemberUse(tg target) []progen.Site {
	uses := map[string][]*syntax.IdentExpr{}
	for _, d := range nodes[*syntax.LetDecl](tg) {
		if d.Value == nil {
			continue
		}
		syntax.Inspect(d.Value, func(n syntax.Node) bool {
			if id, ok := n.(*syntax.IdentExpr); ok {
				uses[id.Name] = append(uses[id.Name], id)
			}
			return true
		})
	}
	var out []progen.Site
	for _, e := range nodes[*syntax.EnumDecl](tg) {
		for _, m := range e.Members {
			if len(uses[m.Name.Name]) != 1 || m.Mods != nil && m.Mods.Retired.Valid() {
				continue
			}
			ms, _ := span(tg, m)
			s, end := span(tg, uses[m.Name.Name][0])
			if onlyUse(tg, e.Name.Name+"."+m.Name.Name, s, end) {
				out = append(out, seq(1, insert(ms, "retired "), mark(tg, s, end)))
			}
		}
	}
	return out
}

// onlyUse tells that the one value-level reference to tg's package's member inside that package (a
// value, loaded data included, a map key or an amendment: API.md R7) spans bytes s..e of tg.
func onlyUse(tg target, member string, s, e int) bool {
	r := projectRefs(tg.project)
	if r == nil {
		return false
	}
	refs, err := r.Member(context.Background(), tg.pkg+":"+member)
	if err != nil {
		return false
	}
	own := slices.DeleteFunc(refs, func(r progen.Ref) bool { return r.Package != tg.pkg || !slices.Contains(valueRefs, r.Kind) })
	return len(own) == 1 && own[0].Span == spanOf(tg, s, e)
}

// spanOf is bytes s..e of tg as the API writes a span (API.md §1.3).
func spanOf(tg target, s, e int) canon.Span {
	sl, sc := position(tg.src, s)
	el, ec := position(tg.src, e)
	return canon.Span{File: tg.path, Line: sl, Col: sc, EndLine: el, EndCol: ec}
}

func position(src []byte, at int) (line, col int) {
	before := src[:at]
	return bytes.Count(before, []byte("\n")) + 1, at - bytes.LastIndexByte(before, '\n')
}

// valueRefs are the reference kinds that hold the member in a value verify judges (E3506).
var valueRefs = []canon.RefKind{canon.RefValue, canon.RefKey, canon.RefLayer}

// openRefs are the reference queries open on the projects targets came from, the corpus and
// shrunk candidates alike, so that a site follows its own project; trouble is the first error
// opening one, which a case with no site reports.
var openRefs = struct {
	sync.Mutex
	by      map[*progen.Project]*progen.Refs
	trouble error
}{by: map[*progen.Project]*progen.Refs{}}

// projectRefs answers reference queries on p, opened once until refsKept projects are open,
// when the whole cache is closed and emptied; nil when it cannot be opened.
func projectRefs(p *progen.Project) *progen.Refs {
	openRefs.Lock()
	defer openRefs.Unlock()
	if r, ok := openRefs.by[p]; ok {
		return r
	}
	if len(openRefs.by) >= refsKept {
		for q, r := range openRefs.by { //canon:unordered closes every cached query; no order reaches an output
			if r != nil {
				_ = r.Close()
			}
			delete(openRefs.by, q)
		}
	}
	r, err := progen.OpenRefs(p, exampleRoots())
	if err != nil && openRefs.trouble == nil {
		openRefs.trouble = err
	}
	openRefs.by[p] = r
	return r
}

// refsTrouble is the reason no reference query could be opened, for a case's failure message.
func refsTrouble() string {
	openRefs.Lock()
	defer openRefs.Unlock()
	if openRefs.trouble == nil {
		return ""
	}
	return ": " + openRefs.trouble.Error()
}

// armed is a match (value or statement level) with its arms and their patterns.
type armed struct {
	match    syntax.Node
	patterns [][]*syntax.Pattern
	arms     []syntax.Node
}

func matches(tg target) []armed {
	var out []armed
	for _, m := range nodes[*syntax.MatchExpr](tg) {
		a := armed{match: m}
		for _, arm := range m.Arms {
			a.patterns, a.arms = append(a.patterns, arm.Patterns), append(a.arms, arm)
		}
		out = append(out, a)
	}
	for _, m := range nodes[*syntax.MatchStmt](tg) {
		a := armed{match: m}
		for _, arm := range m.Arms {
			a.patterns, a.arms = append(a.patterns, arm.Patterns), append(a.arms, arm)
		}
		out = append(out, a)
	}
	return out
}

func hasWildcard(a armed) bool {
	for _, ps := range a.patterns {
		for _, p := range ps {
			if p.Keyword == syntax.TokUnderscore {
				return true
			}
		}
	}
	return false
}

// exhaustivelyMatched are the names the matches without `_` of the corpus name (TYPES.md §12.6).
func exhaustivelyMatched(tg target) map[string]bool {
	out := map[string]bool{}
	for _, other := range *tg.all {
		for _, ps := range matchPatterns(other) {
			if !slices.ContainsFunc(ps, func(p *syntax.Pattern) bool { return p.Keyword == syntax.TokUnderscore }) {
				addPatternNames(out, ps)
			}
		}
	}
	return out
}

// addPatternNames adds the name each of ps names, its last part when qualified.
func addPatternNames(out map[string]bool, ps []*syntax.Pattern) {
	for _, p := range ps {
		if p.Name != nil && len(p.Name.Parts) > 0 {
			out[p.Name.Parts[len(p.Name.Parts)-1].Name] = true
		}
	}
}

// matchPatterns are the patterns of each match of tg, every arm's together, type-level
// matches included.
func matchPatterns(tg target) [][]*syntax.Pattern {
	var out [][]*syntax.Pattern
	for _, a := range matches(tg) {
		out = append(out, slices.Concat(a.patterns...))
	}
	for _, m := range nodes[*syntax.MatchType](tg) {
		var ps []*syntax.Pattern
		for _, arm := range m.Arms {
			ps = append(ps, arm.Patterns...)
		}
		out = append(out, ps)
	}
	return out
}

func dropArm(tg target) []progen.Site {
	var out []progen.Site
	for _, a := range matches(tg) {
		if hasWildcard(a) || len(a.arms) < 2 {
			continue
		}
		for _, arm := range a.arms {
			if startsLine(tg, arm) && endsLine(tg, arm) {
				s, e := span(tg, arm)
				ms, _ := span(tg, a.match)
				out = append(out, seq(0, mark(tg, ms, ms+len("match")), replace(lineStart(tg, s), lineEnd(tg, e)+1, "")))
			}
		}
	}
	return out
}

// addArm adds an arm pattern(first body) => body after the last arm of a match with no _.
func addArm(pattern func(string) string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, a := range matches(tg) {
			last := a.arms[len(a.arms)-1]
			if hasWildcard(a) || !startsLine(tg, last) {
				continue
			}
			ls, le := span(tg, last)
			lastPatterns := a.patterns[len(a.arms)-1]
			if slices.ContainsFunc(lastPatterns, func(p *syntax.Pattern) bool { return p.Binder != nil }) {
				continue // the copied body would read a binder the new pattern lacks
			}
			_, pe := span(tg, lastPatterns[len(lastPatterns)-1])
			body := string(tg.src[pe:le])
			out = append(out, seq(1, insert(le, "\n"+indent(tg, ls)), insert(le, pattern(body)), insert(le, body)))
		}
		return out
	}
}

func repeatPattern(tg target) []progen.Site {
	var out []progen.Site
	for _, a := range matches(tg) {
		if len(a.arms) < 2 || a.patterns[0][0].Binder != nil || a.patterns[0][0].Keyword == syntax.TokUnderscore {
			continue
		}
		first := text(tg, a.patterns[0][0])
		_, pe := span(tg, a.patterns[1][len(a.patterns[1])-1])
		if a.patterns[1][0].Binder != nil {
			continue
		}
		out = append(out, keeping(tg, seq(1, insert(pe, ", "), insert(pe, first)), a.patterns[0][0]))
	}
	return out
}

// dependentPrelude declares an enum, a record and a type function over it.
const dependentPrelude = "local enum ZzK { num, text }\n\nlocal record ZzEv {\n  k: ZzK\n}\n\n" +
	"local type ZzP(e: ZzEv) = match e.k {\n  num => Int\n  text => String\n}\n\nlocal record ZzR {\n"

func dependent(before, focus, after string) func(target) []progen.Site {
	return appendSite(dependentPrelude+"  ev: ZzEv\n  p: ZzP(ev)\n"+before, focus, after+"\n}")
}

func laterField(tg target) []progen.Site {
	return appendSite(dependentPrelude+"  p: ZzP(", "ev", ")\n  ev: ZzEv\n}")(tg)
}

func dependentArity(tg target) []progen.Site {
	return appendSite(dependentPrelude+"  ev: ZzEv\n  p: ", "ZzP(ev, ev)", "\n}")(tg)
}

// neverPrelude declares ZzGK (a Never branch), ZzGP (a third branch too, so a literal of it
// type-checks generically) and ZzGR, whose field p is dependent on it, up to the value a
// mutation gives p for the branch member selects.
func neverPrelude(member string) string {
	return "local enum ZzGK { num, txt, gone }\n\nlocal record ZzGEv {\n  k: ZzGK\n}\n\n" +
		"local type ZzGP(e: ZzGEv) = match e.k {\n  num => Int\n  txt => String\n  gone => Never\n}\n\n" +
		"local record ZzGR {\n  ev: ZzGEv\n  p: ZzGP(ev)\n}\n\nlocal let zzGR: ZzGR = { ev: { k: " + member + " }, p: "
}

// requiredNever gives ZzGR.p a value on its Never branch (TYPES.md §11.6, §13.5).
func requiredNever(tg target) []progen.Site {
	return appendSite(neverPrelude("gone"), "1", " }")(tg)
}

// computedMismatch gives ZzGR.p a String where its branch computes Int (TYPES.md §11.6).
func computedMismatch(tg target) []progen.Site {
	return appendSite(neverPrelude("num"), `"x"`, " }")(tg)
}

// assetHolder declares an asset field over the fixtures' icons and a value giving it focus.
func assetHolder(focus string) func(target) []progen.Site {
	const before = "/// An icon holder.\nlocal record ZzIcon {\n  /// The icon.\n  icon: asset(\"@resource/Icon/Item\", ext: [dds])\n}\n\n" +
		"local let zzIcon: ZzIcon = { icon: "
	return appendSite(before, focus, " }")
}

// spreadNotFirst spreads, after the first item of a table entry, a local copy of that entry;
// only for elements with no ref field, whose copy outside the table would name entries.
func spreadNotFirst(tg target) []progen.Site {
	var out []progen.Site
	withRefs := recordsWithRefs(tg)
	for _, d := range nodes[*syntax.LetDecl](tg) {
		t, ok := d.Type.(*syntax.TableType)
		lit, isLit := d.Value.(*syntax.BraceLit)
		if !ok || !isLit || withRefs[text(tg, t.Name)] {
			continue
		}
		for _, it := range lit.Items {
			e, ok := it.(*syntax.EntryItem)
			if !ok || len(e.Value.Items) < 2 {
				continue
			}
			_, first := span(tg, e.Value.Items[0])
			end := declEnd(tg)
			base := "\n\nlocal let zzBase: " + text(tg, t.Name) + " = " + text(tg, e.Value) + "\n"
			out = append(out, seq(1, insert(first, ", "), insert(first, "...zzBase"), insert(end, base)))
		}
	}
	return out
}

// refOfNonEntry returns, from a function whose result is a ref into a table, a copy of an entry
// that is not one of the table's entries.
func refOfNonEntry(tg target) []progen.Site {
	var out []progen.Site
	for _, d := range nodes[*syntax.LetDecl](tg) {
		t, ok := d.Type.(*syntax.TableType)
		lit, isLit := d.Value.(*syntax.BraceLit)
		if !ok || !isLit || len(lit.Items) == 0 {
			continue
		}
		e, ok := lit.Items[0].(*syntax.EntryItem)
		if !ok {
			continue
		}
		elem := text(tg, t.Name)
		before := "local fn zzPick() -> ref " + d.Name.Name + " { return "
		after := " }\n\nlocal let zzPicked: ref " + d.Name.Name + " = zzPick()"
		out = append(out, appendDecl(tg, before, elem+" "+text(tg, e.Value), after))
	}
	return out
}

// recordsWithRefs are the records of tg's file with a field whose type holds a ref.
func recordsWithRefs(tg target) map[string]bool {
	out := map[string]bool{}
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		for _, it := range r.Body.Items {
			if f, ok := it.(*syntax.FieldDecl); ok && strings.Contains(text(tg, f.Type), "ref ") {
				out[r.Name.Name] = true
			}
		}
	}
	return out
}
