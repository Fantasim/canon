package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// callLangLaw has a named error check translated into French, and poisoned roots of every source
// R6 names: an evaluation's own hard error, an upstream one, and the checker's.
var callLangLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n",
	"a/a.canon": `/// A.
package a

/// A use.
record Use {
  /// How many.
  count: Int

  check big: count < 10 else "count {count} is big"
  warn count < 8 else "count {count} is over eight"
}

/// Uses.
let uses: table Use = {
  first { count: 1 }
}
`,
	"a/a.fr.canon": "package a\ntranslation fr\n\nUse.check.big \"compte {count} trop grand\"\n",
	// Outside what an edit of a re-checks (API.md E17, E18).
	"b/b.canon": `/// B.
package b

/// Own.
let own: Int = [1][2]

/// Upstream.
let total: Int = own + 1

/// A type error.
let typed: Int = "s"

/// A monster.
record M {
  /// Its health.
  hp: Int
}

/// Monsters, one missing its health.
let ms: table M = {
  wolf { hp: 1 }
  bat {}
}
`,
}

// bigEdit sets the one use's count past the check, the edit's re-check then failing it.
func bigEdit(lang string) canon.Edit {
	return canon.Edit{
		Ops: []canon.Op{canon.Set("a:uses.first.count", canon.Int(20))}, DryRun: true,
		Evaluate: []string{"a:uses.first"}, Lang: lang,
	}
}

// openCallLang opens callLangLaw under Options.Lang lang.
func openCallLang(t *testing.T, lang string) *canon.Project {
	t.Helper()
	opts := project(callLangLaw)
	opts.Lang = lang
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// findingMessages is the message of each finding, in order.
func findingMessages(fs []canon.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Message)
	}
	return out
}

// withoutMessages is fs with every message blanked: the finding set R3a keeps across languages.
func withoutMessages(fs []canon.Finding) []canon.Finding {
	out := make([]canon.Finding, 0, len(fs))
	for _, f := range fs {
		f.Message = ""
		out = append(out, f)
	}
	return out
}

// editIn is bigEdit in lang: its result and refusal, which E19 makes a *RejectedError.
func editIn(t *testing.T, p *canon.Project, lang string) (*canon.EditResult, *canon.RejectedError) {
	t.Helper()
	res, err := p.Edit(context.Background(), bigEdit(lang))
	var rej *canon.RejectedError
	if !errors.As(err, &rej) || !errors.Is(err, canon.ErrRejected) || res == nil {
		t.Fatalf("Edit in %q: %v, want a *RejectedError with the result (API.md E19)", lang, err)
	}
	return res, rej
}

// API.md E19, V14, R3a, DECISIONS 345: Edit.Lang words Findings, a refusal's and Eval's; "" is Options.Lang.
func TestEditLang(t *testing.T) {
	const big, grand = "count 20 is big", "compte 20 trop grand"
	for _, c := range []struct {
		name, options, lang, want string
	}{
		{"source", "", "", big},
		{"fr", "", "fr", grand},
		{"options fr", "fr", "", grand},
		{"options fr, en", "fr", "en", big},
		{"unknown", "", "de", big},
	} {
		p := openCallLang(t, c.options)
		base, _ := editIn(t, p, "")
		res, rej := editIn(t, p, c.lang)
		if !reflect.DeepEqual(res.Summary, base.Summary) || !reflect.DeepEqual(withoutMessages(res.Findings), withoutMessages(base.Findings)) {
			t.Errorf("%s: API.md R3a: the finding set or counts changed: %+v vs %+v", c.name, res.Findings, base.Findings)
		}
		if res.Applied || res.Revision != base.Revision {
			t.Errorf("%s: API.md E19: applied %t, revision %s vs %s", c.name, res.Applied, res.Revision, base.Revision)
		}
		if got := findingMessages(rej.Findings); len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: DECISIONS 345: refusal %q, want %q", c.name, got, c.want)
		}
		if got := findingMessages(res.Findings); !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: DECISIONS 345: Findings %q, want %q among them", c.name, got, c.want)
		}
		ev := res.Eval["a:uses.first"]
		if ev == nil || !strings.Contains(strings.Join(findingMessages(ev.Findings), "\n"), c.want) {
			t.Errorf("%s: API.md V14, DECISIONS 345: Eval %+v, want %q", c.name, ev, c.want)
		}
	}
}

// API.md R6a, DECISIONS 345: ValueWith with no Lang is Value; an ErrNoValue wraps the same findings in every language.
func TestValueWithLang(t *testing.T) {
	ctx := context.Background()
	for _, options := range []string{"", "fr"} {
		p := openCallLang(t, options)
		for _, path := range []string{"b:own", "b:total", "b:typed", "b:ms"} {
			noValueIn(t, p, path)
		}
		v, err := p.ValueWith(ctx, canon.ValueRequest{Path: "a:uses", Lang: "fr"})
		if err != nil {
			t.Fatal(err)
		}
		plain, err := p.Value(ctx, "a:uses")
		if err != nil || v.Text != plain.Text || v.Path != plain.Path || !reflect.DeepEqual(v.Origin, plain.Origin) {
			t.Errorf("API.md R6a: a value in fr %+v differs from Value's %+v, %v", v, plain, err)
		}
	}
}

// noValueIn finds path's ErrNoValue, from its own evaluation, upstream or the checker (R6), the
// same in every language: no compiler diagnostic has a translation (I18N.md K8).
func noValueIn(t *testing.T, p *canon.Project, path string) {
	t.Helper()
	ctx := context.Background()
	_, want := p.Value(ctx, path)
	var wp *canon.PathError
	if !errors.As(want, &wp) || !errors.Is(want, canon.ErrNoValue) || len(wp.Findings) != 1 {
		t.Fatalf("Value(%s): %v, want ErrNoValue with its cause (API.md R6)", path, want)
	}
	for _, lang := range []string{"", "en", "fr", "de"} {
		_, got := p.ValueWith(ctx, canon.ValueRequest{Path: path, Lang: lang})
		var gp *canon.PathError
		if !errors.As(got, &gp) || got.Error() != want.Error() || !reflect.DeepEqual(gp.Findings, wp.Findings) {
			t.Errorf("%s in %q: %v %+v, want %v %+v (API.md R6a)", path, lang, got, gp, want, wp.Findings)
		}
	}
}

// API.md E19, R6a, DECISIONS 345: an Edit op or an Evaluate draft refused on a poisoned value is the same in every language.
func TestRefusalLang(t *testing.T) {
	ctx := context.Background()
	p := openCallLang(t, "")
	op := canon.Rename("b:ms.bat", canon.Key("owl"))
	calls := map[string]func(lang string) error{
		"Edit": func(lang string) error {
			_, err := p.Edit(ctx, canon.Edit{AllowErrors: true, Ops: []canon.Op{op}, Lang: lang})
			return err
		},
		"Evaluate": func(lang string) error {
			_, err := p.Evaluate(ctx, canon.EvalRequest{Path: "b:ms", Draft: []canon.Op{op}, Lang: lang})
			return err
		},
	}
	for _, name := range []string{"Edit", "Evaluate"} {
		want := calls[name]("")
		var wp *canon.PathError
		if !errors.Is(want, canon.ErrNoValue) || !errors.As(want, &wp) || len(wp.Findings) == 0 {
			t.Fatalf("%s: %v, want ErrNoValue with its cause (API.md E19, R6)", name, want)
		}
		got := calls[name]("fr")
		var gp *canon.PathError
		if !errors.As(got, &gp) || got.Error() != want.Error() || !reflect.DeepEqual(gp.Findings, wp.Findings) {
			t.Errorf("%s in fr: %v %+v, want %v %+v (API.md R6a)", name, got, gp, want, wp.Findings)
		}
	}
}

// API.md §8.8, DECISIONS 345: an Edit's lang is written as `lang`, omitted when empty, and read back.
func TestEditLangJSON(t *testing.T) {
	for _, c := range []struct {
		lang string
		has  bool
	}{{"fr", true}, {"", false}} {
		data, err := json.Marshal(canon.Edit{Base: "r1:x", Lang: c.lang})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(data), `"lang"`); got != c.has {
			t.Errorf("API.md §8.8: %s, lang written %t, want %t", data, got, c.has)
		}
		var back canon.Edit
		if err := json.Unmarshal(data, &back); err != nil || back.Lang != c.lang || back.Base != "r1:x" {
			t.Errorf("API.md §8.8: %s read back as %+v, %v", data, back, err)
		}
	}
	var e canon.Edit
	if err := json.Unmarshal([]byte(`{"base":"","ops":[],"lang":"fr"}`), &e); err != nil || e.Lang != "fr" {
		t.Errorf("API.md §8.8: lang not read: %+v, %v", e, err)
	}
}
