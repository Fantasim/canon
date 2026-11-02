package fnsize

import "testing"

// fuzzSeeds are the shapes the measurers read: bodies, nesting, literals, templates, arrows.
var fuzzSeeds = []string{
	"void f() {\n  if (a) { b(); } else { c(); }\n}\n",
	"struct S {\n  int a() const { return 1; }\n};\n",
	"template <class T>\nbool Foo<T>::operator==(const Foo& o) const {\n  return true;\n}\n",
	"auto r = R\"x(}\n{\n)x\"; char c = '{'; int k = 1'000;\n#define M(x) \\\n  { x }\n",
	"S::S(int a) : a_{a}, b_(1) {\n  c();\n}\n",
	"const f = (x: number): number => {\n  return x;\n};\nclass C {\n  get v(): number {\n    return 1;\n  }\n}\n",
	"function t<K, B extends Record<string, string> = {}>(rows: K[]): K & { a: number } {\n  return `a${ ({ k: \"}\" }).k }b`;\n}\n",
	"const re = /[{}]\\/x/g; const q = 4 / 2 / 1; foo(function () { a(); });\n",
	"package p\nfunc F() {\n\ta()\n}\n",
	"inline const auto G = [&](int x) mutable -> int { return x; }();\nstruct Rec Make() { if (a) { b(); } }\n",
	"void g() try { a(); } catch (...) { }\nextern \"C\" int f() { }\nalignas(8) struct Y { };\n",
}

// CODEGEN.md §2.11, DECISIONS 435: no input makes a measurer panic or report a negative size.
func FuzzMeasure(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		for _, measure := range []func(string) ([]Func, error){Go, Cpp, TS} {
			funcs, err := measure(src)
			if err != nil {
				continue
			}
			for _, fn := range funcs {
				if fn.Body < 0 || fn.Statements < 0 {
					t.Fatalf("negative size in %+v", fn)
				}
			}
		}
	})
}
