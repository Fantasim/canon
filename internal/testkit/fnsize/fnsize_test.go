package fnsize

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// measureCase is one source and the functions the measurer finds in it, in source order.
type measureCase struct {
	name string
	src  string
	want []Func
}

func lines(l ...string) string { return strings.Join(l, "\n") + "\n" }

// ticks writes the TypeScript template literals of a test source with a placeholder.
func ticks(src string) string { return strings.ReplaceAll(src, "~", "`") }

func runCases(t *testing.T, measure func(string) ([]Func, error), cases []measureCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := measure(c.src)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// CODEGEN.md §2.11, DECISIONS 435: the body is the lines between the braces; go/parser reads Go.
func TestGoBodies(t *testing.T) {
	runCases(t, Go, []measureCase{
		{"three lines", lines("package p", "", "func F() {", "\ta()", "\tb()", "}"), []Func{{"F", 2, 2}}},
		{"method", lines("package p", "func (r *T) M() {", "\ta()", "}"), []Func{{"T.M", 1, 1}}},
		{"empty one-liner", lines("package p", "func G() {}"), []Func{{"G", 0, 0}}},
		{"one statement over many lines", lines("package p", "func L() []int {", "\treturn []int{", "\t\t1,", "\t\t2,", "\t}", "}"),
			[]Func{{"L", 4, 1}}},
		{"braces in strings and comments", lines("package p", "func S() string {", "\t// }", "\treturn \"}{\" + `}", "{`", "}"),
			[]Func{{"S", 3, 1}}},
		{"literal in a package var", lines("package p", "var V = func() {", "\ta()", "}"), []Func{{"V", 1, 1}}},
		{"no body", lines("package p", "func ext()"), nil},
		{"literal in a function is part of it", lines("package p", "func W() {", "\tf := func() {", "\t\ta()", "\t}", "\tf()", "}"),
			[]Func{{"W", 4, 2}}},
	})
}

// CODEGEN.md §2.11, DECISIONS 435: C++ braces in strings, raw strings, comments and character literals are not scopes.
func TestCppBodies(t *testing.T) {
	runCases(t, Cpp, []measureCase{
		{"namespace function", lines("namespace a {", "int f(int x) {", "  return x;", "}", "}"), []Func{{"f", 1, 1}}},
		{"strings comments raw strings", lines("void g() {", "  const char* a = \"}{\";", "  char c = '{';",
			"  auto r = R\"x(}", "{", ")x\";", "  // }", "  /* {", "  } */", "  int k = 1'000;", "}"), []Func{{"g", 9, 4}}},
		{"character literals and digit separators", lines("int d() {", "  int a = 0x1'F;", "  char c = '}';", "  char e = '\\'';",
			"  wchar_t w = L'{';", "  const char* s = u8\"}\";", "  return a;", "}"), []Func{{"d", 6, 6}}},
		{"preprocessor lines", lines("#define OPEN {", "#define M(x) \\", "  { x }", "void h() {", "#if 1", "  a();", "#endif", "}"),
			[]Func{{"h", 3, 1}}},
		{"inline class members", lines("struct S {", "  int a() const { return 1; }", "  void b() {", "    x();", "    y();", "  }", "};"),
			[]Func{{"a", 0, 1}, {"b", 2, 2}}},
		{"lambda counted into its function", lines("int f() {", "  auto g = [&](int x) {", "    return x;", "  };", "  return g(1);", "}"),
			[]Func{{"f", 4, 2}}},
		{"control statements", lines("void k() {", "  if (a) {", "    b();", "  } else {", "    c();", "  }", "  for (;;) { d(); }",
			"  do {", "    e();", "  } while (f());", "}"), []Func{{"k", 9, 3}}},
		{"constructor with an initializer list", lines("S::S(int a) : a_{a}, b_(1) {", "  c();", "}"), []Func{{"S::S", 1, 1}}},
		{"qualified operator of a template", lines("template <class T>", "bool Foo<T>::operator==(const Foo& o) const {", "  return true;", "}"),
			[]Func{{"Foo::operator==", 1, 1}}},
		{"extern block", lines("extern \"C\" {", "void h() {}", "}"), []Func{{"h", 0, 0}}},
		{"data scopes are not functions", lines("enum class E { A, B };", "static const int kT[] = {", "  1,", "  2", "};",
			"int a[3]{1, 2, 3};", "namespace n {}"), nil},
		{"namespace-scope lambda", lines("static const auto kL = []() { return 1; };"), []Func{{"kL", 0, 1}}},
		{"generator initializer lambda", lines("inline const std::vector<int> GROUPS = [] { std::vector<int> v; v.reserve(8); return v; }();"),
			[]Func{{"GROUPS", 0, 3}}},
		{"initializer lambda over many lines, qualifiers, call argument", lines("struct S {", "  static inline const auto k = [&](int x) mutable noexcept -> int {",
			"    return x;", "  }(1);", "};", "const auto w = make([] {", "  a();", "  b();", "});"),
			[]Func{{"k", 1, 1}, {"w", 2, 2}}},
		{"single statement list", lines("static std::vector<int> L() {", "  return {", "    1,", "    2,", "  };", "}"),
			[]Func{{"L", 4, 1}}},
	})
}

// CODEGEN.md §2.11, DECISIONS 435: template literals with ${}; a block-body arrow counts, an expression arrow does not.
func TestTSBodies(t *testing.T) {
	runCases(t, TS, []measureCase{
		{"template literals", ticks(lines("function t(a: string): string {", "  const s = ~a}${ ({ k: \"}\" }).k }b~;",
			"  const n = ~x${ ~y${a}~ }~;", "  return s + n;", "}")), []Func{{"t", 3, 3}}},
		{"multi-line template", ticks(lines("function m(): string {", "  return ~line1", "line2 {", "~;", "}")), []Func{{"m", 3, 1}}},
		{"arrows and object methods", lines("const f = (x: number): number => {", "  return x;", "};", "const g = (x: number) => x + 1;",
			"const h = (x: number) => ({ a: x });", "const o = {", "  m() {", "    a();", "  },", "  p: () => {", "    b();", "    c();", "  },", "};"),
			[]Func{{"f", 1, 1}, {"m", 1, 1}, {"p", 2, 2}}},
		{"class members", lines("export class C {", "  constructor(a: string) {", "    this.a = a;", "  }", "  get v(): number {", "    return 1;", "  }",
			"  static async s<T>(x: T): Promise<T> {", "    return x;", "  }", "}"),
			[]Func{{"constructor", 1, 1}, {"v", 1, 1}, {"s", 1, 1}}},
		{"strings comments regex", lines("function r(s: string): boolean {", "  // }", "  /* { */", "  const a = \"}\";", "  const b = '{';",
			"  const re = /[{}]\\/x/g;", "  const q = 4 / 2 / 1;", "  return re.test(s);", "}"), []Func{{"r", 7, 5}}},
		{"generic default braces", lines("function t<K, B extends Record<string, string> = {}>(rows: K[]): K {", "  return rows[0];", "}"),
			[]Func{{"t", 1, 1}}},
		{"nested arrow is part of its function", lines("function w(xs: number[]): number[] {", "  return xs.map((x) => {", "    return x + 1;", "  });", "}"),
			[]Func{{"w", 3, 1}}},
		{"type literals are not functions", lines("type F = (x: number) => { a: number };", "interface I {", "  a(): void;", "}"), nil},
		{"module statements are not functions", lines("if (x) {", "  y();", "}", "function z() {", "  w();", "}"), []Func{{"z", 1, 1}}},
		{"anonymous function expression", lines("foo(function () {", "  a();", "});"), []Func{{anonymousName, 1, 1}}},
		{"return types with type literals", lines("function a(): X & { readonly [P in keyof B]: (v: T) => T | null } {", "  return x;", "}",
			"function b(): { a: number } {", "  return y;", "}", "function c(): (x: number) => void {", "  return z;", "}"),
			[]Func{{"a", 1, 1}, {"b", 1, 1}, {"c", 1, 1}}},
		{"a class head after a function is not its return type", lines("function f(x: number): number {", "  return x;", "}",
			"const g = (a: number): number => {", "  return a;", "};", "class K<A> extends M<A> {", "  m(): void {", "    a();", "  }", "}"),
			[]Func{{"f", 1, 1}, {"g", 1, 1}, {"m", 1, 1}}},
		{"return type union", lines("export function e(): Foo | null {", "  return null;", "}"), []Func{{"e", 1, 1}}},
	})
}

// CODEGEN.md §2.11, DECISIONS 435: a source the measurer cannot read is an error, never a short list.
func TestReadErrors(t *testing.T) {
	cases := []struct {
		name    string
		measure func(string) ([]Func, error)
		src     string
		want    error
	}{
		{"go syntax", Go, "package p\nfunc (", errParse},
		{"cpp open brace", Cpp, "void f() {\n", errUnbalanced},
		{"cpp close brace", Cpp, "void f() {}}\n", errUnbalanced},
		{"cpp block comment", Cpp, "/* x\n", errUnterminated},
		{"cpp string", Cpp, "const char* s = \"abc\n", errUnterminated},
		{"cpp raw string", Cpp, "auto r = R\"x(abc\n", errUnterminated},
		{"ts open brace", TS, "function f() {\n", errUnbalanced},
		{"ts template", TS, ticks("const a = ~abc\n"), errUnterminated},
		{"ts template expression", TS, ticks("const a = ~${ 1\n"), errUnterminated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.measure(c.src); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// CODEGEN.md §2.11, DECISIONS 435: a C++ header the scanner cannot classify is an error, never a skipped function.
func TestCppRefusals(t *testing.T) {
	cases := []struct{ name, src string }{
		{"elaborated return type", lines("struct Rec Make() {", "  if (a) { b(); }", "}")},
		{"extern with a declaration", lines("extern \"C\" int f() {", "  if (a) { b(); }", "}")},
		{"alignas type", lines("alignas(8) struct Y {", "  int a() { return 1; }", "};")},
		{"alignas inside the head", lines("struct alignas(8) Y {", "  int a() { return 1; }", "};")},
		{"function try block", lines("void g() try {", "  a();", "} catch (...) {", "}")},
		{"control word at declaration level", lines("if (a) {", "  b();", "}")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if funcs, err := Cpp(c.src); !errors.Is(err, errUnclassified) {
				t.Errorf("got %+v, %v; want errUnclassified", funcs, err)
			}
		})
	}
}
