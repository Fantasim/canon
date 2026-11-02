package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const whereByLit = "local record U {\n  x: Int = (T {}).a\n}\n"

// TYPES.md §1, §7.4, §15, DECISIONS 444, 454: a record a constant fold builds has its `where` predicates checked first (its own fields', through a named type, on a list element), so the fold never meets an unchecked predicate; each predicate is reported once, as without the fold.
func TestFoldBuildsWherePredicates(t *testing.T) {
	e3206 := diag.E3206.Def().Code
	for _, tc := range []struct {
		name, text string
		want       []diag.Code
	}{
		{"the record earlier", "package a\n\n" + whereTWith("4") + "\n" + whereByLit, nil},
		{"the record later", "package a\n\n" + whereByLit + "\n" + whereTWith("4"), nil},
		{"a list element", "package a\n\nlocal record T {\n  a: Int = 2\n  l: [Int where it > 0] = [1]\n}\n\nlocal record U {\n  x: Int(1..3) = (T {}).a\n}\n", nil},
		{"a named type", "package a\n\nlocal type Pos = Int where it > 0\n\nlocal record T {\n  a: Int = 2\n  p: Pos = 1\n}\n\nlocal record U {\n  x: Int(1..3) = (T {}).a\n}\n", nil},
		{"a record constant", "package a\n\n" + whereTWith("4") + "\nlocal const C = T {}\n\nlocal record U {\n  x: Int = C.a\n}\n", nil},
		{"values take the defaults", "package a\n\n" + whereTWith("4") + "\n" + whereByLit + "\nlocal let u: U = {}\n\nlocal let t: T = {}\n", nil},
		{"a constant reads the literal", "package a\n\n" + whereTWith("4") + "\nlocal const C = (T {}).a\n\nlocal record U {\n  x: Int = C\n}\n", nil},
		{"a bound reads the literal", "package a\n\n" + whereTWith("4") + "\nlocal let n: Int(0..(T {}).a) = 1\n", nil},
		{"the predicate fails", "package a\n\n" + whereTWith("2") + "\n" + whereByLit, []diag.Code{e3206}},
		{"the predicate fails, a constant reads it", "package a\n\n" + whereTWith("2") + "\nlocal const C = (T {}).a\n\nlocal let n: Int = C\n", []diag.Code{e3206}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := errorCodes(buildOne(t, tc.text).List); !slices.Equal(got, tc.want) {
				t.Errorf("a build reports %v, want %v", got, tc.want)
			}
		})
	}
}

// TYPES.md §1, §7.4, DECISIONS 150, 444, 454: a fold that reports a finding, a storage finding included, breaks its owner: each fault is reported once, and a value of the broken record is W4001, never a second report.
func TestFoldFindingBreaksItsOwner(t *testing.T) {
	e3206, w4001 := diag.E3206.Def().Code, diag.W4001.Def().Code
	for _, tc := range []struct {
		name, text string
		want       []diag.Code
		skipped    bool
	}{
		{"a value takes the default", "package a\n\n" + whereTWith("2") + "\n" + whereByLit + "\nlocal let u: U = {}\n", []diag.Code{e3206}, true},
		{"the literal gives the failing field", "package a\n\nlocal record T {\n  a: Int = 2\n  b: Int where it > 3 = 4\n}\n\n" +
			"local record U {\n  x: Int(1..) = (T { b: 0 }).a\n}\n\nlocal let u: U = {}\n", []diag.Code{e3206}, true},
		{"a bound reads a constant building it", "package a\n\n" + whereTWith("2") + "\nlocal const C = (T {}).a\n\nlocal record U {\n  x: Int(C..) = 9\n}\n", []diag.Code{e3206}, false},
		{"a list element fails, the default is not judged", "package a\n\nlocal record T {\n  a: Int = 5\n  f: [Int where it > 3] = [0]\n}\n\n" +
			"local record U {\n  x: Int(1..3) = (T {}).a\n}\n", []diag.Code{e3206}, false},
		{"a bound reads a constant building it, a value of the broken record", "package a\n\nlocal record T {\n  a: Int = 2\n  b: Int where it > 3 = 0\n}\n\n" +
			"local const C = (T {}).a\n\nlocal record U {\n  x: Int(C..) = 1\n}\n\nlocal let u: U = { x: 5 }\n", []diag.Code{e3206}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list := buildOne(t, tc.text).List
			if got := errorCodes(list); !slices.Equal(got, tc.want) {
				t.Errorf("a build reports %v, want %v", got, tc.want)
			}
			for _, f := range list {
				if f.Code == e3206 && !strings.HasSuffix(f.Message, "where it > 3") {
					t.Errorf("%s reads %q, not the predicate as written", e3206, f.Message)
				}
			}
			if got := slices.Contains(codesOf(list), w4001); got != tc.skipped {
				t.Errorf("%s reported %v, want %v: %v", w4001, got, tc.skipped, codesOf(list))
			}
		})
	}
}

// TYPES.md §1, §7.4, DECISIONS 454: a predicate in error is reported once, at its span, whether or not a fold builds its record first.
func TestFoldBuildsWherePredicateOnce(t *testing.T) {
	const bad = "package shop\n\nlocal record T {\n  a: Int = 5\n  b: Int where it + 1 = 4\n}\n"
	_, alone := checkShopEval(t, bad)
	_, folded := checkShopEval(t, bad+"\n"+whereByLit)
	if alone != folded {
		t.Errorf("a fold changes the predicate's findings:\n%s\nwant\n%s", folded, alone)
	}
}

func whereTWith(def string) string {
	return "local record T {\n  a: Int = 5\n  b: Int where it > 3 = " + def + "\n}\n"
}
