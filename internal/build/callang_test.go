package build_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// langAnalysis is the analysis of the language fixture's package a under Options.Lang lang.
func langAnalysis(t *testing.T, lang string) *build.Analysis {
	t.Helper()
	fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(langA), "p/a/a.fr.canon": file(langFr)}
	p, err := build.Open(fsys, "/p", build.Options{Lang: lang})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// findingSet is every finding of list but its message, sorted: the set a language never changes.
func findingSet(list []diag.Finding) []string {
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, fmt.Sprintf("%s %v %s %s %s %v", f.Code, f.Span, f.Package, f.Path, f.Check, f.Related))
	}
	slices.Sort(out)
	return out
}

// sortedMessages is the messages of list, sorted.
func sortedMessages(list []diag.Finding) []string {
	out := make([]string, 0, len(list))
	for _, f := range list {
		out = append(out, f.Message)
	}
	slices.Sort(out)
	return out
}

// API.md E18, R3a, DECISIONS 345: OwnIn is Own in the language asked, the finding set and counts kept.
func TestOwnIn(t *testing.T) {
	ctx := context.Background()
	src, fr := langAnalysis(t, ""), langAnalysis(t, "fr")
	for _, lang := range []string{"", "en"} {
		own, err := src.OwnIn(ctx, lang)
		if err != nil || !reflect.DeepEqual(own, src.Own()) {
			t.Errorf("OwnIn(%q) is not Own: %v", lang, err)
		}
	}
	own, err := src.OwnIn(ctx, "fr")
	if err != nil {
		t.Fatal(err)
	}
	want := fr.Own()
	if !reflect.DeepEqual(own.Summary, want.Summary) || !slices.Equal(sortedMessages(own.List), sortedMessages(want.List)) {
		t.Errorf("DECISIONS 345: OwnIn(fr) %v, Options.Lang fr %v", sortedMessages(own.List), sortedMessages(want.List))
	}
	if slices.Equal(sortedMessages(own.List), sortedMessages(src.Own().List)) {
		t.Error("DECISIONS 345: OwnIn(fr) translated nothing")
	}
	if !slices.Equal(findingSet(own.List), findingSet(src.Own().List)) {
		t.Error("API.md R3a: OwnIn changed the finding set")
	}
}

// API.md R6a, DECISIONS 345: InOrder words named checks in the language asked and never reorders.
func TestInOrderKeepsOrder(t *testing.T) {
	ctx := context.Background()
	src, fr := langAnalysis(t, ""), langAnalysis(t, "fr")
	given := slices.Clone(src.Source())
	slices.Reverse(given) // against F2 order: a cause comes first whatever its place in F2
	got, err := src.InOrder(ctx, given, "fr")
	if err != nil || !slices.Equal(findingSet(got), findingSet(given)) {
		t.Fatalf("InOrder: %v, %v", got, err)
	}
	for i := range got {
		if fmt.Sprint(got[i].Code, got[i].Span) != fmt.Sprint(given[i].Code, given[i].Span) {
			t.Errorf("API.md R6a: finding %d moved: %s, given %s", i, got[i].Message, given[i].Message)
		}
	}
	if !slices.Equal(sortedMessages(got), sortedMessages(fr.Result().List)) {
		t.Errorf("DECISIONS 345: InOrder(fr) %v, Options.Lang fr %v", sortedMessages(got), sortedMessages(fr.Result().List))
	}
	same, err := fr.InOrder(ctx, fr.Source(), "")
	if err != nil || !slices.Equal(sortedMessages(same), sortedMessages(fr.Result().List)) {
		t.Errorf("API.md R6a: InOrder with no language is not in Options.Lang: %v, %v", sortedMessages(same), err)
	}
}
