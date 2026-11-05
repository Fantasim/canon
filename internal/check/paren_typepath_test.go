package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// parenPathSrc holds a type argument, a scrutinee and two dependent-map domains written in parentheses.
const parenPathSrc = `package a

local enum K { num, text }

local record Ev {
  k: K
}

local type P(e: Ev) = match (e.k) {
  num => Int
  text => String
}

local record T {
  id: String
}

local record Bag {
  subs: [T] keyed by id
}

local let nb: Bag = { subs: [{ id: "x" }] }

local record R {
  ev: Ev
  p: P((ev))
  m: {t in (nb.subs): Int} = {}
}
`

// DECISIONS 171, IMPLEMENTATION-PLAN §4.7: a parenthesis's Types is its content's, in a type path and in a domain.
func TestParenthesisTypesInTypePaths(t *testing.T) {
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", parenPathSrc}})
	if !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	parens := nodesOf[*syntax.ParenExpr](files[0])
	if len(parens) != 3 {
		t.Fatalf("%d parentheses, want 3", len(parens))
	}
	for i, want := range []types.Kind{types.Enum, types.Record, types.List} {
		got := prog.Info.Types[parens[i]]
		if got == nil || got.Base().Kind() != want {
			t.Errorf("Types[paren %d] = %v, want a %v", i, got, want)
		}
		if inside := prog.Info.Types[parens[i].X]; got != inside {
			t.Errorf("Types[paren %d] = %v, its content's is %v", i, got, inside)
		}
	}
}

// DECISIONS 171, IMPLEMENTATION-PLAN §4.7: a parenthesis around a path that names nothing has no Types entry, never a nil one.
func TestParenthesisOfRefusedPathHasNoTypes(t *testing.T) {
	src := strings.Replace(parenPathSrc, "(nb.subs)", "(nb.nope)", 1)
	prog, files, out := checkSet(t, [][2]string{{"a/a.canon", src}})
	if codes, want := renderedErrorCodes(out), []diag.Code{diag.E3003.Def().Code}; !slices.Equal(codes, want) {
		t.Fatalf("error codes %v, want %v:\n%s", codes, want, out)
	}
	for _, p := range nodesOf[*syntax.ParenExpr](files[0]) {
		if got, has := prog.Info.Types[p]; has && got == nil {
			s := files[0].Span(p)
			t.Errorf("Types[%s] is a nil entry", files[0].Src.Content[s.Start:s.End])
		}
	}
}
