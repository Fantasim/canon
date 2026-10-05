package canon_test

import (
	"context"
	"reflect"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// langCheckLaw has a named warning translated into French and an unnamed one.
var langCheckLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n",
	"a/a.canon": `/// A.
package a

/// A use.
record Use {
  /// How many.
  count: Int

  warn big: count < 10 else "count {count} is big"
  warn count < 8 else "count {count} is over eight"
}

/// Uses.
let uses: table Use = {
  first { count: 20 }
}
`,
	"a/a.fr.canon": "package a\ntranslation fr\n\nUse.check.big \"compte {count} trop grand\"\n",
}

// messages is the message of each finding, in order.
func messages(r *canon.CheckResult) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.Message)
	}
	return out
}

// API.md R3a, DECISIONS 313: CheckWith words messages in Lang, the finding set and counts being Check's.
func TestCheckWithLang(t *testing.T) {
	p, err := canon.Open("/law", project(langCheckLaw))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	base, err := p.Check(ctx, "a")
	if err != nil || len(base.Findings) != 2 {
		t.Fatalf("Check: %+v, %v", base, err)
	}
	same, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}})
	if err != nil || !reflect.DeepEqual(messages(same), messages(base)) {
		t.Fatalf("API.md R3a: CheckWith without Lang differs from Check: %v, %v", messages(same), err)
	}
	fr, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base.Summary, fr.Summary) || len(fr.Findings) != len(base.Findings) {
		t.Errorf("API.md R3a, F7: finding set or counts changed: %+v vs %+v", base.Summary, fr.Summary)
	}
	var translated, untouched int
	for i, f := range fr.Findings {
		if f.Code != base.Findings[i].Code || f.Span != base.Findings[i].Span {
			t.Errorf("API.md R3a: finding %d moved: %+v vs %+v", i, f, base.Findings[i])
		}
		if f.Message == "compte 20 trop grand" {
			translated++
		}
		if f.Message == base.Findings[i].Message {
			untouched++
		}
	}
	if translated != 1 || untouched != 1 {
		t.Errorf("API.md R3a: fr messages %v, base %v", messages(fr), messages(base))
	}
}

// API.md R3a: Lang "" is Options.Lang, which Check uses; an unknown language keeps the finding set.
func TestCheckWithLangDefaultsToOptions(t *testing.T) {
	opts := project(langCheckLaw)
	opts.Lang = "fr"
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	base, err := p.Check(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: ""})
	if err != nil || !reflect.DeepEqual(messages(empty), messages(base)) {
		t.Errorf("API.md R3a: Lang \"\" %v, Check %v, %v", messages(empty), messages(base), err)
	}
	found := false
	for _, m := range messages(base) {
		found = found || m == "compte 20 trop grand"
	}
	if !found {
		t.Errorf("API.md R3a: Options.Lang fr not applied: %v", messages(base))
	}
	de, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: "de"})
	if err != nil || len(de.Findings) != len(base.Findings) {
		t.Errorf("API.md R3a: unknown language: %v, %v", de, err)
	}
}
