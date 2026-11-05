package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// optionalRootBase holds a keyed list read through an optional let, through an optional record field, and as an optional last field.
const optionalRootBase = `package a

local record S {
  name: String
  w: Int = 0
}

local record Bag {
  subs: [S] keyed by name
}

local record Out {
  o: Bag?
}

local record Lo {
  l: ([S] keyed by name)?
}

local let nbo: Bag? = { subs: [{ name: "x" }] }

local let out: Out = { o: { subs: [{ name: "x" }] } }

local let lo: Lo = { l: [{ name: "x" }] }
`

// narrowedRead is optionalRootBase with a function returning read where guard is narrowed to present.
func narrowedRead(guard, read string) string {
	return optionalRootBase + "\nlocal fn f() -> Bool {\n  if " + guard + " != none {\n    return " + read + "\n  }\n  return false\n}\n"
}

// TYPES.md §9.4, §10.2, DECISIONS 411: a let path is judged by its fields' declared types, so a narrowed optional step names no let path, as its `!` form names none.
func TestNarrowedOptionalNamesNoLetPath(t *testing.T) {
	unknown, uninferred := []diag.Code{diag.E2102.Def().Code}, []diag.Code{diag.E3008.Def().Code}
	for _, tc := range []struct {
		name, guard, read string
		codes             []diag.Code
	}{
		{"optional middle field, `.k`", "out.o", "out.o.subs.x.w == 0", nil},
		{"optional middle field, hasKey", "out.o", "out.o.subs.hasKey(x)", unknown},
		{"optional middle field, keys()", "out.o", "out.o.subs.keys().contains(out.o.subs.x)", uninferred},
		{"optional last field, `.k`", "lo.l", "lo.l.x.w == 0", nil},
		{"optional last field, hasKey", "lo.l", "lo.l.hasKey(x)", unknown},
		{"optional last field, keys()", "lo.l", "lo.l.keys().contains(lo.l.x)", uninferred},
	} {
		prog, _, out := checkSet(t, [][2]string{{"a/a.canon", narrowedRead(tc.guard, tc.read)}})
		if codes := renderedErrorCodes(out); !slices.Equal(codes, tc.codes) {
			t.Errorf("%s: error codes %v, want %v:\n%s", tc.name, codes, tc.codes, out)
		}
		for _, coll := range prog.Info.Keys { //canon:unordered every entry is judged alone
			if coll.Kind == types.CollLet {
				t.Errorf("%s: a key names the let path %v", tc.name, coll)
			}
		}
	}
}

// TYPES.md §6.2, §10.2, DECISIONS 411: a narrowed optional's keys() is refused at check time, so a build never evaluates it into a false E3503.
func TestNarrowedOptionalKeysNoFalseE3503(t *testing.T) {
	text := narrowedRead("out.o", "out.o.subs.keys().contains(out.o.subs.x)") + "\nlocal let probe: Bool = f()\n"
	want := []diag.Code{diag.E3008.Def().Code}
	if got := errorCodes(buildOne(t, text).List); !slices.Equal(got, want) {
		t.Errorf("a build reports %v, want %v", got, want)
	}
}

// TYPES.md §9.4, §10.2, DECISIONS 411, 477: through `!` or `?.` over an optional let or field, a receiver names the collection the record field holds, never a let path (`ref nbo.subs` is E3504).
func TestKeyedReceiverThroughOptionalRoots(t *testing.T) {
	for _, tc := range []struct{ name, probe string }{
		{"optional let, force", "S = nbo!.subs.x"},
		{"optional let, optional step", "S? = nbo?.subs?.x"},
		{"optional field, force", "S = out.o!.subs.x"},
		{"optional field, optional step", "S? = out.o?.subs?.x"},
		{"optional field, parens", "S = (out.o!).subs.x"},
	} {
		src := optionalRootBase + "\nlocal let probe: " + tc.probe + "\n"
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
		if strings.Contains(out, "error[") {
			t.Errorf("%s: findings:\n%s", tc.name, out)
		}
		for _, g := range gaps(files[0], prog.Info, func(syntax.Decl) bool { return false }) {
			t.Errorf("%s: %s", tc.name, g)
		}
		value, ok := letValue(t, files[0], "probe").(*syntax.SelectorExpr)
		if !ok {
			t.Fatalf("%s: probe's value is no selector", tc.name)
		}
		if got := prog.Info.Keys[value]; got == nil || got.Kind != types.CollField {
			t.Errorf("%s: Keys[%s] = %v, want the field collection a.Bag.subs", tc.name, tc.probe, got)
		}
	}
}

// TYPES.md §9.4, DECISIONS 411, 477: `keys()` of a keyed list an optional holds is a ref into the record field's collection, which no let annotation names (E3008), never `ref nbo.subs`.
func TestKeysThroughOptionalRootNamesNoLetPath(t *testing.T) {
	for _, recv := range []string{"nbo!.subs", "nbo?.subs", "out.o!.subs", "(out.o!).subs"} {
		src := optionalRootBase + "\nlocal let ks = " + recv + ".keys()\n"
		prog, files, _ := checkSet(t, [][2]string{{"a/a.canon", src}})
		if got := prog.Info.Types[letValue(t, files[0], "ks")]; got == nil || strings.Contains(got.String(), "a.nbo") || strings.Contains(got.String(), "a.out") {
			t.Errorf("%s.keys() has type %v, want none of a let path", recv, got)
		}
	}
}
