package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
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

// optionalLetsSrc is optionalRootBase with xs and ot, a keyed list and a table declared optional.
const optionalLetsSrc = optionalRootBase + "\nlocal let xs: ([S] keyed by name)? = [{ name: \"x\" }]\n\nlocal let ot: (table S)? = { x { name: \"x\" } }\n"

// TYPES.md §9.4, DECISIONS 470, 487: an amend path through a let or a field declared optional keys the collection the record field holds, or before any step the optional let's own, the very one its value form's key names (Lo.l, Bag.subs, a.xs), never a let path through a field, for `.k` and `[k]` alike.
func TestAmendKeyThroughOptionalNamesFieldCollection(t *testing.T) {
	for _, tc := range []struct {
		name, block, probe string
		kind               types.CollKind
	}{
		{"optional last field, [k]", "amend lo {\n  l[x].w: 5\n}\n", "lo.l!.x", types.CollField},
		{"optional last field, .k", "amend lo {\n  l.x.w: 5\n}\n", "lo.l!.x", types.CollField},
		{"optional middle field, [k]", "amend out {\n  o.subs[x].w: 5\n}\n", "out.o!.subs.x", types.CollField},
		{"optional middle field, .k", "amend out {\n  o.subs.x.w: 5\n}\n", "out.o!.subs.x", types.CollField},
		{"optional let, [k]", "amend nbo {\n  subs[x].w: 5\n}\n", "nbo!.subs.x", types.CollField},
		{"optional let, .k", "amend nbo {\n  subs.x.w: 5\n}\n", "nbo!.subs.x", types.CollField},
		{"optional keyed-list let, .k", "amend xs {\n  x.w: 5\n}\n", "xs!.x", types.CollLet},
		{"optional table let, .k", "amend ot {\n  x.w: 5\n}\n", "ot!.x", types.CollLet},
	} {
		src := optionalLetsSrc + "\nlocal let probe: S = " + tc.probe + "\n"
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}, {"a/dev.layer.canon", "package a\nlayer dev\n\n" + tc.block}})
		if strings.Contains(out, "error[") {
			t.Errorf("%s: findings:\n%s", tc.name, out)
		}
		for _, f := range files {
			for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
				t.Errorf("%s: %s", tc.name, g)
			}
		}
		got := keyColls(prog.Info, files[1])
		value, ok := letValue(t, files[0], "probe").(*syntax.SelectorExpr)
		if !ok {
			t.Fatalf("%s: probe's value is no selector", tc.name)
		}
		if want := prog.Info.Keys[value]; want == nil || want.Kind != tc.kind || len(got) != 1 || got[0] != want {
			t.Errorf("%s: key collections %v, want one, the collection %v that %s names", tc.name, got, want, tc.probe)
		}
	}
}

// keyColls are the collections Info records for f's bare keys, in Keys and NameKeys.
func keyColls(info *check.Info, f *syntax.File) []*types.Collection {
	var got []*types.Collection
	for _, e := range nodesOf[*syntax.IdentExpr](f) {
		got = append(got, info.Keys[e])
	}
	for _, id := range nodesOf[*syntax.Ident](f) {
		got = append(got, info.NameKeys[id])
	}
	return slices.DeleteFunc(got, func(c *types.Collection) bool { return c == nil })
}

// TYPES.md §9.4, DECISIONS 477: `x!` holds the collection x holds, so through a keyed-list or table let declared optional, `.k` after `!` or `?.` keys the let's own collection.
func TestForcedOptionalLetKeysItsCollection(t *testing.T) {
	for _, tc := range []struct{ let, probe, recv string }{
		{"xs", "Int = xs!.x.w", "xs!.x"},
		{"xs", "Int? = xs?.x?.w", "xs?.x"},
		{"xs", "Int = (xs!).x.w", "(xs!).x"},
		{"ot", "Int = ot!.x.w", "ot!.x"},
	} {
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", optionalLetsSrc + "\nlocal let probe: " + tc.probe + "\n"}})
		if strings.Contains(out, "error[") {
			t.Errorf("%s: findings:\n%s", tc.probe, out)
		}
		for _, g := range gaps(files[0], prog.Info, func(syntax.Decl) bool { return false }) {
			t.Errorf("%s: %s", tc.probe, g)
		}
		read, ok := letValue(t, files[0], "probe").(*syntax.SelectorExpr)
		if !ok {
			t.Fatalf("%s: probe's value is no selector", tc.probe)
		}
		recv, ok := read.X.(*syntax.SelectorExpr)
		if !ok {
			t.Fatalf("%s: the read's receiver is no selector", tc.probe)
		}
		if got := prog.Info.Keys[recv]; got == nil || got.Kind != types.CollLet || got.Name != tc.let || len(got.FieldPath) != 0 {
			t.Errorf("%s: Keys[%s] = %v, want the collection of the let %s", tc.probe, tc.recv, got, tc.let)
		}
	}
}
