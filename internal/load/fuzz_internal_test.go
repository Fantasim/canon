package load

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// csvSeeds adds every case of the callfindings CSV suite, and a few edge cases, to the corpus.
func csvSeeds(f *testing.F) {
	f.Helper()
	for _, s := range []string{
		"", "a,b\n1,2\n", "a,\"b\"\"c\"\n", "a,\"b\nc\"\n", "\xef\xbb\xbfa,b\n",
		"a,b\"c\n", "a,\"bc\n", "a,\"b\"c\n", "a,b\nc\n", "\xff\xfe",
		"\"a\"\rb\n", "\"a\"\r", "a\rb\n", "a,\"x\r\ny\"\r\nc,d\r\n", "\r\r\n",
	} {
		f.Add([]byte(s))
	}
}

// IMPLEMENTATION-PLAN.md §7.7: the CSV reader never panics, every span inside the file.
func FuzzParseCSV(f *testing.F) {
	csvSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		set := &source.FileSet{}
		bag := diag.NewBag(set, "p")
		src, err := set.Add("a.csv", "/p/a.csv", data)
		if err != nil {
			return // FileSet.Add's own size limit, not the reader's concern
		}
		req := Request{Pkg: "p", Bag: bag}
		start, ok := checkUTF8(src, data, "a.csv", req)
		if !ok {
			checkSpans(t, bag, src)
			return
		}
		if _, ok := parseCSV(src, data, start, req); !ok && len(bag.Findings()) == 0 {
			t.Fatalf("not ok with no finding")
		}
		checkSpans(t, bag, src)
	})
}

// definesSeeds adds every case of the callfindings defines suite, and a few edge cases.
func definesSeeds(f *testing.F) {
	f.Helper()
	for _, s := range []string{
		"", "#define X 1\n", "#define X 1\n#define X 2\n", "#define F(x) x\n",
		"#define X (1 << 4) | 0x2\n", "/* a\nb */\n#define X 1\n", "#define X \\\n1\n",
		"#ifndef X\n#define X\n#endif\n", "\x00\xff",
		"#error don't\n#define X 1\n", "#define S \"a\\\nb\"\n", "#define X -(-(1)) << 2\n",
		"#define X 1\n#define Y ((X)\n", "/* \\\n*/ #define X 1\n",
	} {
		f.Add([]byte(s))
	}
}

// IMPLEMENTATION-PLAN.md §7.7: the #define reader never panics, every span inside the file.
func FuzzReadDefines(f *testing.F) {
	definesSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		set := &source.FileSet{}
		bag := diag.NewBag(set, "p")
		src, err := set.Add("a.h", "/p/a.h", data)
		if err != nil {
			return
		}
		req := Request{Pkg: "p", Bag: bag}
		readDefines(src, req)
		checkSpans(t, bag, src)
	})
}

// checkSpans is every finding of bag, all inside src, none from another file.
func checkSpans(t *testing.T, bag *diag.Bag, src *source.File) {
	t.Helper()
	size := source.Pos(len(src.Content))
	for _, fd := range bag.Findings() {
		if s := fd.Span; s.File != src.ID || s.Start > s.End || s.End > size {
			t.Fatalf("%s has span %+v outside the file (size %d)", fd.Code, s, size)
		}
	}
}
