package progen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// comparedWithName is the text after a renamed field compared with a bare lower-case name.
var comparedWithName = regexp.MustCompile(`^ (==|!=) [a-z_]+\b[^.(\[]`)

const comparedLookahead = 40 // bytes enough to hold " == name" and the byte after it

// The operators the progen nightly of 2026-11-01 found writing a second finding of their own, or
// none: each with a fixture of the construct it must now leave alone and of one it still mutates.
func TestMutatorsAvoidNightlySites(t *testing.T) {
	const goEmit = "emit go { out: \"o/\" }\n"
	for _, tc := range []struct {
		rule, src string
		sites     func(target) []progen.Site
		want      []string // each site's focus text, in order
	}{
		{
			"WIRE.md §4.2: no member none in an enum already holding the wire \"none\" (a second wire)",
			"package m\nenum E { a, b = \"none\" }\nenum F { c }\n",
			noneMember, []string{"none"},
		},
		{
			"TYPES.md §6.6: no rename of a tested path, whose readers lose their narrowing",
			"package m\nrecord C {\n  v: Int?\n  w: Int\n}\nfn f(c: C) -> Int { return if c.v != none { c.v + c.w } else { 0 } }\n",
			unknownMember, []string{"zzv", "zzw"},
		},
		{
			"TYPES.md §4.3, §11.4: no member renamed on a dependent value (another code)",
			"package m\nenum V { x, y }\nenum X { p }\nenum Y { q }\ntype M(v: V) = match v {\n  x => X\n  y => Y\n}\n" +
				"record L { n: String }\nfn f(v: V) -> [L] { return M(v).members.map(m => L { n: m.name }) }\n",
			unknownMember, []string{"zzmap"},
		},
		{
			"TYPES.md §15: only a range in a type's arguments is a refinement bound",
			"package m\nrecord R { n: Int(..=3) }\nlet xs: [Int] = [1, 2]\nlet ys: [Int] = xs[..1]\n",
			boundReadsLet, []string{"zzLimit"},
		},
		{
			"TYPES.md §10.3: no live copy of a retired entry (a live reference to it)",
			"package m\nrecord P {\n  a: Int\n  b: Int\n}\nlet ps: table P = {\n  one { a: 1, b: 2 }\n  retired two { a: 1, b: 2 }\n}\n",
			spreadNotFirst, []string{"...zzBase"},
		},
		{
			"TYPES.md §3.7: a local record is never emitted, so it has no Go name to refuse",
			"package m\nlocal record L { n: Int }\nrecord P { n: Int }\n" + goEmit,
			badGoName, []string{"P"},
		},
		{
			"WIRE.md §5.9, CODEGEN.md §4.4: no Range field in a record a @text fn's result reaches, which refuses it too (no wire form)",
			"package m\nrecord S { n: Int }\nrecord R { s: [S] }\nrecord Q { n: Int }\n@text(\"r.json\")\nexport fn r() -> R { return { s: [] } }\n" + goEmit,
			goRecordField("zzSpan", ": Range = 0..1"), []string{"zzSpan"},
		},
		{
			"CODEGEN.md §4.4: a Range field in a go-emitting package with no @text fn",
			"package m\nrecord R { n: Int }\n" + goEmit,
			goRecordField("zzSpan", ": Range = 0..1"), []string{"zzSpan"},
		},
		{
			"CODEGEN.md §5.10, §5.13, DECISIONS 289: no package fn where a data emit refuses it too (a second refusal)",
			"package m\nemit go { out: \"o/\", mode: types }\nemit ts { out: \"o.ts\", mode: data }\n",
			withoutData(withMode("", modeTypes, "/// An answer.\nexport fn ", "zzAnswer", "() -> Int { return 42 }")), nil,
		},
		{
			"CODEGEN.md §5.10: a TS embedded emit is baked, so it refuses no package fn",
			"package m\nemit go { out: \"o/\", mode: types }\nemit ts { out: \"o.ts\", mode: embedded }\n",
			withoutData(withMode("", modeTypes, "/// An answer.\nexport fn ", "zzAnswer", "() -> Int { return 42 }")), []string{"zzAnswer"},
		},
		{
			"TYPES.md §6.6: `x!.f` is no stable path, so no `!` on the root of a tested path",
			"package m\nrecord C { v: Int? }\nfn f(c: C, n: Int) -> Int { return if c.v != none { c.v! + n } else { n } }\n",
			forceParameter, []string{"n!", "n!"},
		},
		{
			"TYPES.md §6.6: a path compared with a value, not tested against none, is still renamed",
			"package m\nenum K { ts, id }\nrecord C { kind: K }\nfn f(c: C) -> Bool { return c.kind == id }\n",
			unknownMember, []string{"zzkind"},
		},
		{
			"TYPES.md §11, §16: a type argument reading a field is a use of it",
			"package m\nenum A { g }\ntype B(a: A) = match a {\n  g => Int\n}\nrecord E {\n  aim: A\n  b: B(aim)\n}\n",
			deprecatedFieldUse, []string{"aim"},
		},
	} {
		var got []string
		for _, s := range tc.sites(fixtureTarget(tc.src)) {
			got = append(got, editTexts(s)[s.Focus])
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s: focus texts %q, want %q", tc.rule, got, tc.want)
		}
	}
}

// TYPES.md §1, §5.1: each unknown-member site compared with a bare name (`c.kind == id`) reports alone.
func TestUnknownMemberComparedWithName(t *testing.T) {
	c := examples(t)
	o := op(diag.E3003.Def().Code, "TYPES.md §3.5 (unknown member)", unknownMember)
	n := 0
	for _, p := range o.placements(c) {
		src, _ := c.project.Get(p.site.Path)
		at := p.site.Edits[0]
		if !comparedWithName.Match(src[at.End:min(len(src), at.End+comparedLookahead)]) {
			continue
		}
		n++
		m, err := p.site.Mutate(c.project)
		if err != nil {
			t.Fatalf("%s: %v", p.site.Path, err)
		}
		if v := judge(o, p.run, m.At, m.Project, c.baselineOf(p.run.pkgs)); v.Kind != "" {
			t.Errorf("%s %q: %s", p.site.Path, at.Text, v.Text)
		}
	}
	if n == 0 {
		t.Fatal("no site compares a renamed field with a bare name")
	}
	t.Logf("%d sites", n)
}
