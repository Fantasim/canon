package build_test

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	langProject = "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"
	langA       = `/// A.
package a

/// A use.
record Use {
  /// How many.
  count: Int

  warn big: count < 10 else "count {count} is big"
  warn alpha: count < 5 else "a: {count} is over five"
  warn zeta: count < 6 else "z: {count} is over six"
  warn untranslated: count < 7 else "count {count} is over seven"
  warn count < 8 else "count {count} is over eight"
  warn broken: count < 9 else "count {count} is over nine"
  check {
    if count > 9 {
      warn(count, "block {count}")
    }
  }
}

/// Uses.
let uses: table Use = {
  first { count: 20 }
}

check total: uses.len() == 2 else "uses has {uses.len()} entries"
`
	langFr = `package a
translation fr

Use.check.big "compte {count} trop grand"
Use.check.alpha "z : {count} dépasse cinq"
Use.check.zeta "a : {count} dépasse six"
Use.check.broken "compte {nothing}"
check.total "uses a {uses.len()} entrées"
`
)

// langResult is `canon check a` of the language fixture under --lang lang.
func langResult(t *testing.T, lang string) *build.Result {
	t.Helper()
	fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(langA), "p/a/a.fr.canon": file(langFr)}
	p, err := build.Open(fsys, "/p", build.Options{Lang: lang})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// langCheck is langResult's findings.
func langCheck(t *testing.T, lang string) []diag.Finding {
	t.Helper()
	return langResult(t, lang).List
}

// API.md F4, F5: a translated finding keeps every field of the source one but its message.
func TestLangKeepsFields(t *testing.T) {
	located := func(lang string) []any {
		res := langResult(t, lang)
		i := slices.IndexFunc(res.List, func(f diag.Finding) bool { return f.Check == "big" })
		if i < 0 {
			t.Fatalf("no finding of check big in %+v", res.List)
		}
		l := diag.Locate(res.Files, res.List[i:i+1])[0]
		return []any{l.Code, l.Severity, l.Loc, l.Pointer, l.Package, l.Path, l.Check, l.Layer, l.Related, l.Stack, l.MoreFrames, l.Reads}
	}
	if src, fr := located(""), located("fr"); !reflect.DeepEqual(src, fr) {
		t.Errorf("French finding %+v, want %+v", fr, src)
	}
}

// messageOf is the message of the finding of check name, "" for an unnamed one's of code.
func messageOf(t *testing.T, list []diag.Finding, name string, code diag.Code) string {
	t.Helper()
	i := slices.IndexFunc(list, func(f diag.Finding) bool { return f.Check == name && f.Code == code })
	if i < 0 {
		t.Fatalf("no %s finding of check %q in %+v", code, name, list)
	}
	return list[i].Message
}

// I18N.md B5, K8, T2, T3: under --lang fr a named one-line check's message is its translation,
// rendered with the same arguments; an untranslated or broken one, an unnamed one and a block
// form's report stay in the source language.
func TestLangCheckMessages(t *testing.T) {
	w1, w2, e1 := diag.W5001.Def().Code, diag.W5002.Def().Code, diag.E5001.Def().Code
	list := langCheck(t, "fr")
	for _, c := range []struct {
		name string
		code diag.Code
		want string
	}{
		{"big", w1, "compte 20 trop grand"},
		{"alpha", w1, "z : 20 dépasse cinq"},
		{"zeta", w1, "a : 20 dépasse six"},
		{"total", e1, "uses a 1 entrées"},
		{"untranslated", w1, "count 20 is over seven"},
		{"broken", w1, "count 20 is over nine"},
		{"", w2, "block 20"},
	} {
		if got := messageOf(t, list, c.name, c.code); got != c.want {
			t.Errorf("check %q: message %q, want %q", c.name, got, c.want)
		}
	}
	unnamed := slices.IndexFunc(list, func(f diag.Finding) bool { return f.Check == "" && f.Message == "count 20 is over eight" })
	if unnamed < 0 {
		t.Errorf("the unnamed check's message is not in the source language: %+v", list)
	}
}

// I18N.md B5, K8; CLI.md §2.3: the source language, an unknown code and no --lang agree.
func TestLangFallback(t *testing.T) {
	want := langCheck(t, "")
	for _, lang := range []string{"en", "xx"} {
		got := langCheck(t, lang)
		if !slices.EqualFunc(got, want, func(a, b diag.Finding) bool { return a.Message == b.Message && a.Check == b.Check }) {
			t.Errorf("--lang %s: %+v, want %+v", lang, got, want)
		}
	}
	if messageOf(t, want, "big", diag.W5001.Def().Code) != "count 20 is big" {
		t.Errorf("source messages %+v", want)
	}
}

// API.md F2: findings are sorted by their translated message; alpha's and zeta's warnings share
// a span and a code, and their French messages sort the other way round.
func TestLangSortsTranslated(t *testing.T) {
	order := func(list []diag.Finding) []string {
		var out []string
		for _, f := range list {
			if f.Check == "alpha" || f.Check == "zeta" {
				out = append(out, f.Check)
			}
		}
		return out
	}
	if got := order(langCheck(t, "")); !slices.Equal(got, []string{"alpha", "zeta"}) {
		t.Errorf("source order %v", got)
	}
	if got := order(langCheck(t, "fr")); !slices.Equal(got, []string{"zeta", "alpha"}) {
		t.Errorf("French order %v", got)
	}
}

// I18N.md B5, VIEWMODEL.md J15: `canon build --lang fr` reports the translated message, while the
// view model keeps `message` in the source language and the translation in `messages`.
func TestLangBuildKeepsModelSource(t *testing.T) {
	p, err := build.Open(viewFS(), "/p", build.Options{Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	if err != nil {
		t.Fatal(err)
	}
	if got := messageOf(t, res.List, "big", diag.W5001.Def().Code); got != "compte 20 trop grand" {
		t.Errorf("build finding %q", got)
	}
	m := decodeModel(t, viewOutput(t, res, "@out/a.view.json").Content)
	i := slices.IndexFunc(m.Findings, func(f vm.Finding) bool { return f.Check == "big" })
	if i < 0 {
		t.Fatalf("no finding of check big in %+v", m.Findings)
	}
	if f := m.Findings[i]; f.Message != "count 20 is big" || !maps.Equal(f.Messages, map[string]string{"fr": "compte 20 trop grand"}) {
		t.Errorf("model finding %q, messages %v", f.Message, f.Messages)
	}
}

// I18N.md B5, §11: the farm's `unreachable_levels` warning, `at` a field, in French.
func TestLangFarm(t *testing.T) {
	res, err := openExamplesWith(t, t.TempDir(), build.Options{Lang: "fr"}).Check(context.Background(), []string{"resource.farm"})
	if err != nil {
		t.Fatal(err)
	}
	want := "maxLevel vaut 3 mais seuls 2 paliers existent : le niveau 3 et au-delà sont inatteignables (GetLevel renvoie nullptr)"
	if got := messageOf(t, res.List, "unreachable_levels", diag.W5001.Def().Code); got != want {
		t.Errorf("message %q, want %q", got, want)
	}
}

// I18N.md B5, IMPLEMENTATION-PLAN §7.6: a warm check replaying the memo translates as a cold one.
func TestLangCached(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(langA), "p/a/a.fr.canon": file(langFr)}
	p, err := build.Open(fsys, "/p", build.Options{Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithCache(build.NewCache())
	cold := langCheck(t, "fr")
	for round := range 2 {
		res, err := p.Check(context.Background(), []string{"a"})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.EqualFunc(res.List, cold, func(a, b diag.Finding) bool { return a.Message == b.Message && a.Check == b.Check }) {
			t.Errorf("round %d: %+v, want %+v", round, res.List, cold)
		}
	}
}

// API.md F7, DECISIONS 281: with MaxFindings reached, the kept finding is chosen on the source
// messages, so --lang fr keeps alpha's (whose French message sorts last) as no --lang does.
func TestLangTruncationKeepsSourceChoice(t *testing.T) {
	const src = "/// A.\npackage a\n\n/// A use.\nrecord Use {\n  /// How many.\n  count: Int\n\n" +
		"  warn alpha: count < 5 else \"a: {count} is over five\"\n" +
		"  warn zeta: count < 6 else \"z: {count} is over six\"\n}\n\n" +
		"/// Uses.\nlet uses: table Use = {\n  first { count: 20 }\n}\n"
	const fr = "package a\ntranslation fr\n\nUse.check.alpha \"z : {count} dépasse cinq\"\nUse.check.zeta \"a : {count} dépasse six\"\n"
	kept := func(lang string) []string {
		fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(src), "p/a/a.fr.canon": file(fr)}
		p, err := build.Open(fsys, "/p", build.Options{Lang: lang, MaxFindings: 1})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Check(context.Background(), []string{"a"})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range res.List {
			if f.Package == "a" {
				out = append(out, f.Check+": "+f.Message)
			}
		}
		return out
	}
	if got := kept(""); !slices.Equal(got, []string{"alpha: a: 20 is over five"}) {
		t.Errorf("source kept %q", got)
	}
	if got := kept("fr"); !slices.Equal(got, []string{"alpha: z : 20 dépasse cinq"}) {
		t.Errorf("French kept %q", got)
	}
}

// sharedSite's two instances come from one record literal, so they share a site (EVALUATION.md §8.3).
const sharedSite = `/// A.
package a

/// A use.
record Use {
  /// How many.
  count: Int

  warn big: count < 10 else "MESSAGE"
}

/// Makes a use.
fn mk(n: Int) -> Use { return Use { count: n } }

/// Bs.
let b: [Use] = [mk(30)]

/// As.
let a: [Use] = [mk(20)]

emit view { out: "@out/a.view.json" }
`

// sharedFS is sharedSite with the check's message src and its French translation fr.
func sharedFS(src, fr string) mapFS {
	return mapFS{
		"p/project.canon": file(viewProject), "p/a/a.canon": file(strings.Replace(sharedSite, "MESSAGE", src, 1)),
		"p/a/a.fr.canon": file("package a\ntranslation fr\n\nUse.check.big \"" + fr + "\"\n"),
	}
}

// sharedBuild builds sharedFS(src, fr) under --lang fr: each finding's path to its message, and the model's.
func sharedBuild(t *testing.T, src, fr string) (got, model map[string]string) {
	t.Helper()
	p, err := build.Open(sharedFS(src, fr), "/p", build.Options{Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	if err != nil {
		t.Fatal(err)
	}
	got, model = map[string]string{}, map[string]string{}
	for _, f := range res.List {
		if f.Check == "big" {
			got[f.Path] = f.Message
		}
	}
	for _, f := range decodeModel(t, viewOutput(t, res, "@out/a.view.json").Content).Findings {
		if f.Check == "big" {
			model[f.Path] = f.Messages["fr"]
		}
	}
	return got, model
}

// I18N.md T3, VIEWMODEL.md J15, DECISIONS 281: of two instances sharing a site and a source
// message, the finding kept renders the translation with its own values, in both outputs.
func TestLangOwnInstance(t *testing.T) {
	want := map[string]string{"a[0]": "compte 20 trop grand"}
	got, model := sharedBuild(t, "too big", "compte {count} trop grand")
	if !maps.Equal(got, want) || !maps.Equal(model, want) {
		t.Errorf("findings %v, model messages %v, want %v", got, model, want)
	}
}

// EVALUATION.md §14, DECISIONS 281: two findings whose translations read alike stay two.
func TestLangDuplicatesOnSource(t *testing.T) {
	want := map[string]string{"a[0]": "trop grand", "b[0]": "trop grand"}
	if got, _ := sharedBuild(t, "count {count} too big", "trop grand"); !maps.Equal(got, want) {
		t.Errorf("findings %v, want %v", got, want)
	}
}

// I18N.md B5, LOCK.md §5, API.md O7: no canon.lock byte changes when a French message forces b.x.
func TestLangLockBytes(t *testing.T) {
	const src = "/// A.\npackage a\n\nimport b\n\n/// A use.\nrecord Use {\n  /// How many.\n  count: Int\n}\n\n" +
		"/// Uses.\nlet uses: stable table Use = {\n  first { count: 20 }\n  second { count: 2 }\n}\n\n" +
		"warn few: uses.len() > 5 else \"only {uses.len()} uses\"\n"
	const other = "/// B.\npackage b\n\n/// Read by a's French message only.\nlet x: Int = 7\n"
	const fr = "package a\ntranslation fr\n\ncheck.few \"seulement {uses.len()} usages, b vaut {b.x}\"\n"
	lock := func(lang string) (string, string) {
		fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(src), "p/a/a.fr.canon": file(fr), "p/b/b.canon": file(other)}
		p, err := build.Open(fsys, "/p", build.Options{Lang: lang})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}})
		if err != nil {
			t.Fatal(err)
		}
		written := fsys["p/a/canon.lock"]
		if written == nil || len(res.Locks) != 1 {
			t.Fatalf("--lang %q: no canon.lock written: %+v", lang, res.Locks)
		}
		return string(written.Data), messageOf(t, res.List, "few", diag.W5001.Def().Code)
	}
	plain, srcMsg := lock("")
	french, frMsg := lock("fr")
	if plain != french {
		t.Errorf("canon.lock differs under --lang fr:\n%s\nwant\n%s", french, plain)
	}
	if srcMsg != "only 2 uses" || frMsg != "seulement 2 usages, b vaut 7" {
		t.Errorf("messages %q, %q", srcMsg, frMsg)
	}
}
