package check_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
)

// fmtCase is a variant whose second case's bound builds its first, the second's default v.
func fmtCase(v string) string {
	return fmt.Sprintf(siblingCase, v)
}

// fmtHalf is a record B whose default builds A giving field.
func fmtHalf(field string) string {
	return fmt.Sprintf(halfBuiltB, field)
}

// The outcomes at HEAD (87c8e1e3) a row's head records: unchanged, or an internal compiler error.
const (
	headSame  = "same"
	headICE   = "internal error"
	headCycle = "no finding, a build's emit an internal error"
)

const (
	nestedRecords = "\nlocal record T {\n  u: U = {}\n}\n\nlocal record U {\n  y: Int = 4\n}\n"
	importedPkg   = "package b\n\nrecord T {\n  u: U = {}\n}\n\nrecord U {\n  y: Int = 4\n}\n"
	refTable      = "\nlocal record S {\n  m: Int\n}\n\nlocal let ss: table S = {\n  open { m: 1 }\n}\n"
	appliedTypes  = "\nlocal enum K { num, text }\n\nlocal record Ev {\n  k: K\n}\n\nlocal type P(e: Ev) = match e.k {\n  num => Int\n  text => String\n}\n" +
		"\nlocal record L(e: Ev) {\n  id: String\n  v: P(e)\n  u: U = {}\n}\n\nlocal record R {\n  ev: Ev\n  ls: [L(ev)]\n}\n\nlocal record U {\n  y: Int = 4\n}\n"
	brokenDefault = "package shop\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int = NOPE\n}\n"
	siblingCase   = "\nlocal variant V {\n  one {\n    n: Int = 2\n  }\n  two {\n    m: Int(0..(V.one {}).n) = %s\n  }\n}\n"
	halfBuiltB    = "\nlocal record B {\n  x: Int = 3\n  y: Int = (A { %s: 5 }).c\n}\n"
	cycleK        = "package a\n\nconst K = (T {}).a\n\nrecord T {\n  a: Int = K\n}\n"
	cycleY        = "local const Y = (T {}).a\n\nlocal record T {\n  a: Int = Y\n}\n"
	cycleZ        = "\nrecord R {\n  r: Int(..=100) = Z\n}\n\nconst Z = (T {}).a + 0\n\nconst Y = (T {}).a\n\nrecord T {\n  a: Int = Y\n}\n"
	cycleK1K2     = "\nconst K1 = (T1 {}).a\n\nconst K2 = (T2 {}).b\n\nrecord T1 {\n  a: Int = K2\n}\n\nrecord T2 {\n  b: Int = K1\n}\n"
	laterCaseText = "\nlocal variant V {\n  two {\n    m: Int(%s) = %s\n  }\n  one %s\n}\n"
)

// laterCase is a variant whose first case's bound builds its second, declared later.
func laterCase(bound, value, body string) string {
	return fmt.Sprintf(laterCaseText, bound, value, body)
}

// foldRecordCases fold a refinement bound or a default that builds a record or a case before its
// body's turn; head is what HEAD gave, want every error a build reports now.
var foldRecordCases = []struct {
	name, head, text string
	want             []*diag.Def
}{
	{"a later record literal", headSame, "package a\n\nlocal let n: Int(0..(T { x: 3 }).x) = 1\n\nlocal record T {\n  x: Int\n}\n", nil},
	{"a later record's default", headSame, "package a\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int = 5\n}\n", nil},
	{"a nested record's default", headICE, "package a\n\nlocal let n: Int(0..(T {}).u.y) = 1\n" + nestedRecords, nil},
	{"an earlier nested record", headICE, "package a\n" + nestedRecords + "\nlocal let n: Int(0..(T {}).u.y) = 1\n", nil},
	{"a nested record, the value outside", headICE, "package a\n\nlocal let n: Int(0..(T {}).u.y) = 9\n" + nestedRecords, []*diag.Def{diag.E3204.Def()}},
	{"a const building a nested record", headICE, "package a\n\nlocal const K = (T {}).u.y\n\nlocal let n: Int(0..K) = 1\n" + nestedRecords, nil},
	{"a record-valued const", headICE, "package a\n\nconst K = T {}\n\nlocal let n: Int(0..K.u.y) = 1\n" +
		"\nrecord T {\n  u: U = {}\n}\n\nrecord U {\n  y: Int = 4\n}\n", nil},
	{"a bound in a fn body", headICE, "package a\n\nlocal fn f() -> Int {\n  let y: Int(0..(T {}).u.y) = 1\n  return y\n}\n" + nestedRecords, nil},
	{"a bound in a field type", headICE, "package a\n\nlocal record A {\n  a: Int(0..(T {}).u.y) = 1\n}\n" + nestedRecords, nil},
	{"a default reading a const that builds a record", headICE, "package a\n\nlocal const K = (T {}).u.y\n\nlocal record A {\n  a: Int(..=0) = K\n}\n" + nestedRecords,
		[]*diag.Def{diag.E3204.Def()}},
	{"a variant case's default", headSame, "package a\n\nlocal let n: Int(0..(V.one {}).n) = 1\n\nlocal variant V {\n  one {\n    n: Int = 2\n  }\n  two\n}\n", nil},
	{"a variant case's nested default", headICE, "package a\n\nlocal let n: Int(0..(V.one {}).u.y) = 1\n\nlocal variant V {\n  one {\n    u: U = {}\n  }\n  two\n}\n" +
		"\nlocal record U {\n  y: Int = 4\n}\n", nil},
	{"a bound building a finished sibling case", headSame, "package a\n" + fmtCase("1"), nil},
	{"a bound building a finished sibling case, the value outside", headSame, "package a\n" + fmtCase("5"), []*diag.Def{diag.E3204.Def()}},
	{"a let's bound, then a sibling case's", headSame, "package a\n\nlocal let n: Int(0..(V.one {}).n) = 1\n" + fmtCase("1"), nil},
	{"an applied record's default", headICE, "package a\n\nlocal let n: Int(0..(R { ev: { k: num }, ls: [{ id: \"a\", v: 1 }] }).ls[0].u.y) = 1\n" + appliedTypes, nil},
	{"a ref field's default", headSame, "package a\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int = 3\n  s: ref S = \"open\"\n}\n" + refTable, nil},
	{"a ref list field's default", headSame, "package a\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int = 3\n  s: [ref S] = []\n}\n" + refTable, nil},
	{"a later default naming nothing", headICE, "package a\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int = NOPE\n}\n", []*diag.Def{diag.E2102.Def()}},
	{"a nested default naming nothing", headICE, "package a\n\nlocal let n: Int(0..(T {}).u.y) = 1\n\nlocal record T {\n  u: U = {}\n}\n\nlocal record U {\n  y: Int = NOPE\n}\n",
		[]*diag.Def{diag.E2102.Def()}},
	{"a later default outside its refinement", "an empty range, the default's range finding twice", "package a\n\nlocal let n: Int(0..(T {}).x) = 1\n\nlocal record T {\n  x: Int(1..) = 0\n}\n", []*diag.Def{diag.E3204.Def()}},
	{"a bound building its own record", headSame, "package a\n\nlocal record T {\n  x: Int = 1\n  y: Int(0..(T {}).x) = 0\n}\n", []*diag.Def{diag.E3015.Def()}},
	{"a bound building its own record, a later field read", headICE, "package a\n\nlocal record A {\n  a: Int(0..(A { c: 5 }).c) = 1\n  c: Int = 2\n}\n", []*diag.Def{diag.E3015.Def()}},
	{"a bound reading a const building its own record", headICE, "package a\n\nlocal record A {\n  a: Int(0..K) = 1\n  c: Int = 2\n}\n\nlocal const K = (A { c: 5 }).c\n",
		[]*diag.Def{diag.E3015.Def()}},
	{"a bound building a record whose default builds the bound's", headSame, "package a\n\nlocal record A {\n  a: Int(0..(B {}).x) = 1\n}\n" +
		"\nlocal record B {\n  x: Int = [A {}].len()\n}\n", []*diag.Def{diag.E3015.Def()}},
	{"a record whose fields are unresolved, read by a default", headICE, "package a\n\nlocal record A {\n  a: Int(0..(B {}).x) = 1\n  c: Int = 2\n}\n" +
		fmtHalf("c"), []*diag.Def{diag.E3015.Def()}},
	{"a record whose bound field is unresolved, read by a default", headICE, "package a\n\nlocal record A {\n  c: Int = 2\n  a: Int(0..(B {}).x) = 1\n}\n" +
		fmtHalf("a"), []*diag.Def{diag.E3015.Def()}},
	{"a record whose fields are unresolved, in a list default", headICE, "package a\n\nlocal record T {\n  x: Int = 3\n  us: [U] = [U { y: 0 }]\n}\n" +
		"\nlocal record U {\n  y: Int(0..(T {}).x) = 1\n}\n", []*diag.Def{diag.E3015.Def()}},
	{"a bound building a record that names the bound's", headSame, "package a\n\nlocal record A {\n  a: Int(0..(B {}).x) = 1\n}\n" +
		"\nlocal record B {\n  x: Int = 3\n  y: A?\n}\n", nil},
	{"a const cycle through a default", headCycle, cycleK, []*diag.Def{diag.E4301.Def()}},
	{"a const cycle through a default, a bound on the const", headICE, cycleK + "\nlet n: Int(0..K) = 1\n", []*diag.Def{diag.E4301.Def()}},
	{"a cycle of two consts through a default", headCycle, "package a\n\nconst K = K2 + 1\n\nconst K2 = (T {}).a\n\nrecord T {\n  a: Int = K\n}\n",
		[]*diag.Def{diag.E4301.Def()}},
	{"a bound building the record of a const cycle", headICE, "package a\n\nlocal let n: Int(0..(T {}).a) = 1\n\n" + cycleY, []*diag.Def{diag.E4301.Def()}},
	{"a const cycle, then a bound building its record", headICE, "package a\n\n" + cycleY + "\nlocal let n: Int(0..(T {}).a) = 1\n", []*diag.Def{diag.E4301.Def()}},
	{"a default folding a const that builds a cycle's record", headICE, "package a\n" + cycleZ, []*diag.Def{diag.E4301.Def()}},
	{"a bound reaching a two-const cycle through two defaults", headICE, "package a\n\nlocal let n: Int(0..(T2 {}).b) = 1\n" + cycleK1K2, []*diag.Def{diag.E4301.Def()}},
	{"a bound building a sibling case declared later", headSame, "package a\n" + laterCase("0..[V.one {}].len()", "1", "{\n    n: Int = 2\n  }"),
		[]*diag.Def{diag.E3204.Def()}},
	{"a bound building a sibling case declared later, inclusive", headSame, "package a\n" + laterCase("0..=[V.one {}].len()", "0", "{\n    n: Int = 2\n  }"), nil},
	{"a bound building a bodiless sibling case declared later", headSame, "package a\n" + laterCase("0..=[V.one {}].len()", "0", ""), nil},
	{"a literal giving the defaulted field is no cycle", headSame, "package a\n\nconst K = (T { a: 1 }).a\n\nrecord T {\n  a: Int = K\n}\n", nil},
}
