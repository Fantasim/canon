package progen_test

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// crashKinds are the failures no input may cause, whatever else a case reports.
var crashKinds = []string{kindCrash, kindPanic, kindInternal, kindHang}

// bornOf is the born line of a counterexample of suite first failing with sig: the class its
// signature's kind falls in, then the signature (decision log "Progen archives after a fix").
func bornOf(suite, sig string) string {
	kind, _, _ := strings.Cut(sig, " ")
	class := classProperty
	switch {
	case slices.Contains(crashKinds, kind):
		class = kind
	case suite == suiteMutation:
		class = classMismatch
	}
	return class + " " + sig
}

// bornSig is the signature of a born line.
func bornSig(born string) string {
	_, sig, _ := strings.Cut(born, " ")
	return sig
}

// standsFor tells a kept counterexample of suite and name that stands for a failure signed sig:
// its current failure's or the one it was born with.
func standsFor(c *progen.Counterexample, suite, name, sig string) bool {
	if c.Suite != suite || c.Name != name {
		return false
	}
	return sigKey(c.Sig) == sigKey(sig) || sigKey(bornSig(c.Born)) == sigKey(sig)
}

// findingSet counts a run's findings by code and message, the places messages name cut.
type findingSet map[string]int

// reportedBy is the set of fs.
func reportedBy(fs []progen.Finding) findingSet {
	s := findingSet{}
	for _, f := range fs {
		s[findingKey(f)]++
	}
	return s
}

// covers tells that s holds every finding of fs, as many times as fs does.
func (s findingSet) covers(fs []progen.Finding) bool {
	for key, n := range reportedBy(fs) { //canon:unordered a count per key; no order reaches an output
		if n > s[key] {
			return false
		}
	}
	return true
}

func findingKey(f progen.Finding) string {
	return string(f.Code) + " " + rePlace.ReplaceAllString(f.Message, "_")
}

// bornVerdict judges a replayed mutation archive on the bug it was born with only: Kind "" once
// it is gone, whatever the shrunk program's leftovers make the compiler report besides (doc.go).
// Under a crash guard, the expected code is not judged either.
func (j judged) bornVerdict(code diag.Code, at progen.Region, born, guard string) verdict {
	v := j.verdict(code, at)
	class, sig, _ := strings.Cut(born, " ")
	switch {
	case v.Kind == "":
		return v
	case !validGuard(class, guard) || !slices.Contains(crashKinds, class) && class != classMismatch:
		return verdict{Kind: kindMalformed, Sig: kindMalformed, Text: "malformed born or guard line: " + born + "; " + guard}
	case slices.Contains(crashKinds, v.Kind):
		return v
	case guard == kindCrash:
		return verdict{Sig: leftovers(j.other), Text: "fixed, but for the crash guard's leftovers: " + v.Text}
	case len(j.hit) == 0 || class == classMismatch && !j.bornGone(sig):
		return v
	}
	return verdict{Sig: leftovers(j.other), Text: "fixed, but for the leftovers: " + v.Text}
}

// leftovers are the shapes of fs, sorted, each with its count when above one: "E1117*2, W1002".
func leftovers(fs []progen.Finding) string {
	counts := map[string]int{}
	for _, f := range fs {
		counts[shapeOf(f.Code, f.Message)]++
	}
	out := make([]string, 0, len(counts))
	for _, shape := range slices.Sorted(maps.Keys(counts)) {
		if counts[shape] > 1 {
			shape += countSep + strconv.Itoa(counts[shape])
		}
		out = append(out, shape)
	}
	return strings.Join(out, shapeSep)
}

// leftCounts reads a leftovers list back into its counts.
func leftCounts(list string) map[string]int {
	counts := map[string]int{}
	for _, item := range strings.Split(list, shapeSep) {
		shape, n, many := strings.Cut(item, countSep)
		count, err := strconv.Atoi(n)
		if !many || err != nil {
			count = 1
		}
		if shape != "" {
			counts[shape] += count
		}
	}
	return counts
}

// withinLeft is the verdict of a fixed archive whose born bug is gone (v, its leftovers in
// v.Sig): it fails with every shape reported more often than its left line allows, none when
// it has none (strict).
func withinLeft(v verdict, left string) verdict {
	have, allowed := leftCounts(v.Sig), leftCounts(left)
	var over []string
	for _, shape := range slices.Sorted(maps.Keys(have)) {
		if have[shape] > allowed[shape] {
			over = append(over, shape)
		}
	}
	if len(over) == 0 {
		return v
	}
	sig := kindExtra + " " + strings.Join(over, shapeSep)
	return verdict{Kind: kindExtra, Sig: sig, Text: "findings beyond its left line (" + left + "): " + v.Text}
}

// validGuard tells a guard line an archive born of class may carry: none, or "crash" on a
// crash-born one.
func validGuard(class, guard string) bool {
	return guard == "" || guard == kindCrash && slices.Contains(crashKinds, class)
}

// bornGone tells that the mismatch a case was born with is gone: none of the codes its birth
// reported besides the expected one is reported off the site, in any variant; born repeated,
// the expected code is reported once.
func (j judged) bornGone(sig string) bool {
	kind, shapeList, _ := strings.Cut(sig, " ")
	switch kind {
	case kindRepeated:
		return len(j.hit) == 1
	case kindError:
		return true
	}
	var codes []diag.Code
	for _, shape := range strings.Split(shapeList, shapeSep) {
		code, _, _ := strings.Cut(shape, variantSep)
		codes = append(codes, diag.Code(code))
	}
	return !slices.ContainsFunc(j.other, func(f progen.Finding) bool { return slices.Contains(codes, f.Code) })
}

// Decision log "Progen archives after a fix": an archive is judged on its own bug only; a
// different bug, a crash or the born defect in another variant still fails it; a crash guard
// judges the crash alone, and only on a crash-born archive.
func TestBornVerdict(t *testing.T) {
	want, cascaded := diag.E3003.Def().Code, diag.E3002.Def().Code
	at := progen.Region{Start: 10, End: 20}
	hit := progen.Finding{Code: want, Path: "a/a.canon", Start: 12}
	leftover := progen.Finding{Code: diag.W1002.Def().Code, Path: "a/a.canon", Start: 1, Message: "m1"}
	cascade := progen.Finding{Code: cascaded, Path: "a/a.canon", Start: 40, Message: "m2"}
	mismatch := func(kind string, codes ...diag.Code) string {
		shapes := make([]string, 0, len(codes))
		for _, c := range codes {
			shapes = append(shapes, string(c))
		}
		return strings.TrimSpace(classMismatch + " " + kind + " " + strings.Join(shapes, shapeSep))
	}
	crashed := kindInternal + " " + kindInternal + " boom"
	for _, tc := range []struct {
		name, born, guard string
		j                 judged
		fixed             bool
	}{
		{"crash guard, no crash, code missing", crashed, kindCrash, judged{other: []progen.Finding{leftover}}, true},
		{"crash guard, still crashing", crashed, kindCrash, judged{out: progen.Outcome{Panic: "panic: x\n"}}, false},
		{"crash guard on a mismatch", mismatch(kindExtra, cascaded), kindCrash, judged{hit: []progen.Finding{hit}, other: []progen.Finding{cascade}}, false},
		{"unknown guard", crashed, kindHang, judged{other: []progen.Finding{leftover}}, false},
		{"crash-born, leftovers only", crashed, "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{leftover, cascade}}, true},
		{"crash-born, still crashing", crashed, "", judged{out: progen.Outcome{Panic: "panic: x\n"}}, false},
		{"crash-born, wanted code missing", kindPanic + " " + kindPanic + " x in eval.f", "", judged{other: []progen.Finding{leftover}}, false},
		{"mismatch-born, leftovers only", mismatch(kindExtra, cascaded), "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{leftover}}, true},
		{"mismatch-born, the born extra stays", mismatch(kindExtra, cascaded+variantSep+"other", diag.W1003.Def().Code), "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{cascade}}, false},
		{"mismatch-born missing, reported now", mismatch(kindMissing, diag.E3008.Def().Code), "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{leftover}}, true},
		{"mismatch-born missing, still missing", mismatch(kindMissing), "", judged{other: []progen.Finding{leftover}}, false},
		{"born repeated, reported once", mismatch(kindRepeated, want), "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{leftover}}, true},
		{"born repeated, still twice", mismatch(kindRepeated, want), "", judged{hit: []progen.Finding{hit, hit}, other: []progen.Finding{leftover}}, false},
		{"passes outright", mismatch(kindExtra, cascaded), "", judged{hit: []progen.Finding{hit}}, true},
		{"malformed born line", classProperty + " idempotence", "", judged{hit: []progen.Finding{hit}, other: []progen.Finding{leftover}}, false},
	} {
		if v := tc.j.bornVerdict(want, at, tc.born, tc.guard); (v.Kind == "") != tc.fixed {
			t.Errorf("%s: verdict %q (%s), want fixed %v", tc.name, v.Kind, v.Text, tc.fixed)
		}
	}
}

// Decision log (review of the QA progen unit): a fixed archive reports no finding beyond its
// left line, shape by shape and count by count; with no left line, none at all.
func TestLeftLine(t *testing.T) {
	w, e := diag.W1002.Def().Code, diag.E3002.Def().Code
	doc := progen.Finding{Code: w, Message: "m1"}
	bad := progen.Finding{Code: e, Message: "m2"}
	left := leftovers([]progen.Finding{doc, doc, bad})
	if want := string(e) + shapeSep + string(w) + countSep + "2"; left != want {
		t.Fatalf("leftovers = %q, want %q", left, want)
	}
	for _, tc := range []struct {
		name, left string
		now        []progen.Finding
		pass       bool
	}{
		{"the recorded leftovers", left, []progen.Finding{doc, bad, doc}, true},
		{"fewer leftovers", left, []progen.Finding{doc}, true},
		{"one more of a recorded shape", left, []progen.Finding{doc, doc, bad, bad}, false},
		{"a new shape", string(w), []progen.Finding{bad}, false},
		{"strict, clean", "", nil, true},
		{"strict, any finding", "", []progen.Finding{doc}, false},
	} {
		v := withinLeft(verdict{Sig: leftovers(tc.now)}, tc.left)
		if (v.Kind == "") != tc.pass {
			t.Errorf("%s: verdict %q %q, want pass %v", tc.name, v.Kind, v.Sig, tc.pass)
		}
	}
}

// A born line names the class its signature's kind falls in; a leftover never enters a shrunk
// mutation, whose findings stay among the unshrunk case's, places in messages aside.
func TestBornOfAndCovers(t *testing.T) {
	extra := kindExtra + " " + string(diag.E3002.Def().Code)
	for _, tc := range []struct{ suite, sig, want string }{
		{suiteMutation, extra, classMismatch + " " + extra},
		{suiteMutation, "hang in eval.loop", "hang hang in eval.loop"},
		{suiteGrammar, "crash fatal error: stack overflow in check.f", "crash crash fatal error: stack overflow in check.f"},
		{suiteCorrupt, "idempotence in File/FnDecl", "property idempotence in File/FnDecl"},
	} {
		if got := bornOf(tc.suite, tc.sig); got != tc.want || bornSig(got) != tc.sig {
			t.Errorf("bornOf(%s, %q) = %q, want %q", tc.suite, tc.sig, got, tc.want)
		}
	}
	e := diag.E2106.Def().Code
	base := reportedBy([]progen.Finding{{Code: e, Message: "m1 (a/a.canon:3:1)"}})
	moved := progen.Finding{Code: e, Message: "m1 (a/a.canon:2:1)"}
	if !base.covers([]progen.Finding{moved}) || base.covers([]progen.Finding{{Code: e, Message: "m2"}}) {
		t.Error("covers must ignore the places a message names and nothing else")
	}
	if base.covers([]progen.Finding{moved, moved}) {
		t.Error("covers must count: a second finding of one message is a leftover")
	}
}
