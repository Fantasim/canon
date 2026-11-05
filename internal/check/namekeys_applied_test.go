package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// appliedRootBase holds a keyed list in a record field typed by an applied record, `sub: Sub(k)`,
// and the same list behind a ref field, `r: ref es`.
const appliedRootBase = `package a

local enum K { x, y }

local record T {
  id: String
}

local record Sub(k: K) {
  kind: K = k
  subs: [T] keyed by id
}

local record Holder {
  k: K
  sub: Sub(k)
}

local let h: Holder = { k: x, sub: { subs: [{ id: "a" }] } }

local record E {
  id: String
  subs: [T] keyed by id
}

local let es: [E] keyed by id = [{ id: "e", subs: [{ id: "a" }] }]

local record RH {
  r: ref es
}

local let rh: RH = { r: e }
`

// probeKey is the bare key `a` a probe writes, or the probe's selector `.a` when it writes none.
func probeKey(f *syntax.File, probe syntax.Expr) syntax.Expr {
	ids := nodesOf[*syntax.IdentExpr](f)
	if last := ids[len(ids)-1]; last.Name == "a" {
		return last
	}
	return probe
}

// TYPES.md §9.4, §10.2, §11.1, DECISIONS 411: a field typed by an applied record is a required record, so the path names the let path's collection.
func TestKeyedReceiverThroughAppliedRecord(t *testing.T) {
	for _, tc := range []struct {
		name, probe string
		codes       []diag.Code
	}{
		{"`.k` (TYPES.md §9.1)", "probe: T = h.sub.subs.a", nil},
		{"index (STDLIB.md §5)", "probe: T = h.sub.subs[a]", nil},
		{"hasKey (STDLIB.md §5)", "probe: Bool = h.sub.subs.hasKey(a)", nil},
		{"contains takes an element (DECISIONS 317)", "probe: Bool = h.sub.subs.contains(a)", []diag.Code{diag.E3026.Def().Code}},
	} {
		src := appliedRootBase + "\nlocal let " + tc.probe + "\n"
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
		if codes := renderedErrorCodes(out); !slices.Equal(codes, tc.codes) {
			t.Errorf("%s: error codes %v, want %v:\n%s", tc.name, codes, tc.codes, out)
		}
		got := prog.Info.Keys[probeKey(files[0], letValue(t, files[0], "probe"))]
		if got == nil || got.Kind != types.CollLet || got.String() != "a.h.sub.subs" {
			t.Errorf("%s: Keys[a] = %v, want the let path a.h.sub.subs", tc.name, got)
		}
	}
	src := appliedRootBase + "\nlocal let ks = h.sub.subs.keys()\n"
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
	if !strings.HasPrefix(out, noFindings) {
		t.Errorf("keys(): findings:\n%s", out)
	}
	if got := prog.Info.Types[letValue(t, files[0], "ks")]; got == nil || got.String() != "[ref a.h.sub.subs]" {
		t.Errorf("h.sub.subs.keys() has type %v, want [ref a.h.sub.subs]", got)
	}
}

// TYPES.md §9.1, §9.4, §10.2: through a ref field a path names no let path, so a bare key is unknown and keys() uninferred; `.a` keys the field's collection.
func TestKeyedReceiverThroughRefField(t *testing.T) {
	unknown := []diag.Code{diag.E2102.Def().Code}
	for _, tc := range []struct {
		name, probe string
		codes       []diag.Code
	}{
		{"index", "probe: T = rh.r.subs[a]", unknown},
		{"hasKey", "probe: Bool = rh.r.subs.hasKey(a)", unknown},
		{"keys()", "probe = rh.r.subs.keys()", []diag.Code{diag.E3008.Def().Code}},
		{"`.k`", "probe: T = rh.r.subs.a", nil},
	} {
		src := appliedRootBase + "\nlocal let " + tc.probe + "\n"
		prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
		if codes := renderedErrorCodes(out); !slices.Equal(codes, tc.codes) {
			t.Errorf("%s: error codes %v, want %v:\n%s", tc.name, codes, tc.codes, out)
		}
		if tc.codes != nil {
			continue
		}
		got := prog.Info.Keys[letValue(t, files[0], "probe")]
		if got == nil || got.Kind != types.CollField {
			t.Errorf("%s: Keys[.a] = %v, want the field collection a.E.subs", tc.name, got)
		}
	}
}

// renderedErrorCodes is the codes of the errors a rendering lists, in order.
func renderedErrorCodes(out string) []diag.Code {
	var codes []diag.Code
	for line := range strings.SplitSeq(out, "\n") {
		if code, ok := strings.CutPrefix(line, "error["); ok {
			codes = append(codes, diag.Code(code[:strings.IndexByte(code, ']')]))
		}
	}
	return codes
}
