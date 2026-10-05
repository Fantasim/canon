package canon_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md R3a, F7, DECISIONS 281, 313: with Options.Lang "fr", CheckWith in the source language
// gives the source messages, the same set and counts; "fr" and Check are unchanged.
func TestCheckWithSourceLangUnderOptionsLang(t *testing.T) {
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
	en, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	got := messages(en)
	slices.Sort(got)
	if want := []string{"count 20 is big", "count 20 is over eight"}; !slices.Equal(got, want) {
		t.Errorf("API.md R3a: source-language messages %v, want %v", got, want)
	}
	if !reflect.DeepEqual(en.Summary, base.Summary) || len(en.Findings) != len(base.Findings) {
		t.Errorf("API.md F7, DECISIONS 281: set or counts changed: %+v vs %+v", en.Summary, base.Summary)
	}
	fr, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: "fr"})
	if err != nil || !reflect.DeepEqual(messages(fr), messages(base)) {
		t.Errorf("DECISIONS 313: Lang fr %v, Check %v, %v", messages(fr), messages(base), err)
	}
	again, err := p.Check(ctx, "a")
	if err != nil || !reflect.DeepEqual(messages(again), messages(base)) {
		t.Errorf("DECISIONS 313: Check changed by CheckWith: %v vs %v, %v", messages(again), messages(base), err)
	}
	de, err := p.CheckWith(ctx, canon.CheckRequest{Packages: []string{"a"}, Lang: "de"})
	if err != nil || len(de.Findings) != len(base.Findings) {
		t.Fatalf("API.md R3a: %v, %v", de, err)
	}
	if slices.Contains(messages(de), "compte 20 trop grand") {
		t.Errorf("API.md R3a: unknown language kept Options.Lang words: %v", messages(de))
	}
}
