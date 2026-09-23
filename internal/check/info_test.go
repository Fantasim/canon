package check_test

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

const planPath = "../../spec/IMPLEMENTATION-PLAN.md"

// IMPLEMENTATION-PLAN §4.7: an *Ident or *IdentExpr reaches its object; nothing else does.
func TestObjectOf(t *testing.T) {
	status := &object{kind: check.ObjTypeName, name: "Status", pkg: "teamboard", typ: types.StringType}
	limit := &object{kind: check.ObjLet, name: "limit", pkg: "teamboard", typ: types.IntType}
	def, typeName, use := &syntax.Ident{Name: "limit"}, &syntax.Ident{Name: "Status"}, &syntax.IdentExpr{Name: "limit"}
	key, annotation, lit := &syntax.IdentExpr{Name: "taken"}, &syntax.Ident{Name: "json"}, &syntax.IntLit{}
	info := &check.Info{
		Defs:     map[*syntax.Ident]check.Object{def: limit},
		NameUses: map[*syntax.Ident]check.Object{typeName: status},
		Uses:     map[*syntax.IdentExpr]check.Object{use: limit},
		Keys:     map[syntax.Expr]*types.Collection{key: {Name: "statuses"}},
	}
	for _, tc := range []struct {
		name string
		n    syntax.Node
		want check.Object
	}{
		{"declaring identifier", def, limit},
		{"type name", typeName, status},
		{"name in value position", use, limit},
		{"symbolic key", key, nil},
		{"identifier naming no object", annotation, nil},
		{"not an identifier", lit, nil},
		{"recovery node", &syntax.BadExpr{}, nil},
	} {
		if got := info.ObjectOf(tc.n); got != tc.want {
			t.Errorf("%s: ObjectOf = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// IMPLEMENTATION-PLAN §4.7: the kinds print the sketch's names in its order, NoneIndex is its value.
func TestKindNamesFollowThePlan(t *testing.T) {
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	section := sketch(t, string(plan))
	for _, tc := range []struct {
		typ   string
		names []string
	}{
		{"ObjKind", names(check.ObjConst, check.ObjLet, check.ObjFn, check.ObjMethod, check.ObjParam,
			check.ObjLocal, check.ObjField, check.ObjMember, check.ObjCase, check.ObjEntry, check.ObjBuiltin,
			check.ObjTypeName, check.ObjPackage, check.ObjLayer, check.ObjCheck, check.ObjTest, check.ObjWidget)},
		{"SelKind", names(check.SelField, check.SelEntry, check.SelBuiltinMember, check.SelMethod)},
		{"ConvKind", names(check.ConvWrap, check.ConvDeref, check.ConvEntryToRef, check.ConvIntLitToFloat,
			check.ConvCaseToVariant, check.ConvToList, check.ConvElements, check.ConvPresent)},
		{"CalleeKind", names(check.CalleeFn, check.CalleeMethod, check.CalleeBuiltin, check.CalleeConvert, check.CalleeLambda)},
		{"LitKind", names(check.LitRecord, check.LitTable, check.LitMap, check.LitMapComp, check.LitError)},
	} {
		if want := kindList(section, tc.typ); !slices.Equal(tc.names, want) {
			t.Errorf("%s: code prints %v, §4.7 lists %v", tc.typ, tc.names, want)
		}
	}
	if want := "const NoneIndex = " + strconv.Itoa(check.NoneIndex) + "\n"; !strings.Contains(section, want) {
		t.Errorf("§4.7 does not declare %q", want)
	}
}

// names prints the kinds, which must be consecutive from 0, then the value past the last.
func names[K interface {
	~uint8
	String() string
}](kinds ...K) []string {
	var out []string
	for i, k := range kinds {
		if int(k) != i {
			return nil
		}
		out = append(out, k.String())
	}
	return append(out, K(len(kinds)).String())
}

// sketch is the Go block of §4.7.
func sketch(t *testing.T, plan string) string {
	t.Helper()
	start, end := strings.Index(plan, "### 4.7 "), strings.Index(plan, "### 4.8 ")
	if start < 0 || end < start {
		t.Fatal("§4.7 not found in " + planPath)
	}
	return plan[start:end]
}

var (
	kindLine = regexp.MustCompile(`^type (\w+) uint8\s*//(.*)$`)
	contLine = regexp.MustCompile(`^\s+//(.*)$`)
	kindWord = regexp.MustCompile(`^[A-Z][A-Za-z]*$`)
)

// kindList reads the names listed after `type <typ> uint8`, glosses dropped, then "".
func kindList(section, typ string) []string {
	var text strings.Builder
	in := false
	for line := range strings.SplitSeq(section, "\n") {
		if m := kindLine.FindStringSubmatch(line); m != nil {
			in = m[1] == typ
			if in {
				text.WriteString(m[2])
			}
			continue
		}
		m := contLine.FindStringSubmatch(line)
		if !in || m == nil {
			in = false
			continue
		}
		text.WriteString(" " + m[1])
	}
	var out []string
	for _, w := range strings.FieldsFunc(dropGlosses(text.String()), func(r rune) bool { return r == ',' || r == ' ' }) {
		if kindWord.MatchString(w) {
			out = append(out, w)
		}
	}
	return append(out, "")
}

func dropGlosses(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '(':
			depth++
		case r == ')':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}
