package jsonsrc_test

import (
	"bytes"
	"errors"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

// seeds adds the example sources, the findings cases and a few edge cases to a fuzz corpus.
func seeds(f *testing.F) {
	f.Helper()
	sources := exampleSources(f)
	for _, name := range slices.Sorted(maps.Keys(sources)) {
		f.Add(sources[name])
	}
	cases, err := filepath.Glob("testdata/findings/*.txtar")
	if err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		a, err := txtar.ParseFile(c)
		if err != nil {
			f.Fatal(err)
		}
		for _, file := range a.Files {
			if path.Ext(file.Name) == ".json" {
				f.Add(file.Data)
			}
		}
	}
	for _, s := range []string{"", "\xef\xbb\xbf[1]", `"\ud800"`, `{"a~/": -0.5e+10}`, "\xff\xfe"} {
		f.Add([]byte(s))
	}
}

// IMPLEMENTATION-PLAN.md §7.7: no panic; sound spans, pointers and findings.
func FuzzParse(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		p := parse(t, string(data))
		size := source.Pos(len(p.file.Content))
		for _, fd := range p.findings {
			if s := fd.Span; s.File != p.file.ID || s.Start > s.End || s.End > size {
				t.Fatalf("%s has span %+v outside the file", fd.Code, s)
			}
		}
		var enc *jsonsrc.EncodingError
		switch {
		case p.err == nil:
			checkNode(t, p, p.root, source.Span{File: p.file.ID, End: size}, "")
			wantCodes(t, p, "")
		case errors.As(p.err, &enc):
			if enc.Span.Start > enc.Span.End || enc.Span.End > size {
				t.Fatalf("encoding error %+v outside the file", enc.Span)
			}
			wantCodes(t, p, "")
		case errors.Is(p.err, jsonsrc.ErrSyntax):
			wantCodes(t, p, diag.E7109.Def().Code)
		case errors.Is(p.err, jsonsrc.ErrDuplicateKey):
			wantCodes(t, p, diag.E7104.Def().Code)
		default:
			t.Fatalf("unexpected error %v", p.err)
		}
	})
}

// wantCodes checks that every finding has the code, and that there is one iff code is set.
func wantCodes(t *testing.T, p parsed, code diag.Code) {
	t.Helper()
	if (code == "") != (len(p.findings) == 0) || code == diag.E7109.Def().Code && len(p.findings) != 1 {
		t.Fatalf("%v: findings %v, want %q", p.err, p.findings, code)
	}
	for _, fd := range p.findings {
		if fd.Code != code {
			t.Fatalf("finding %s, want %s", fd.Code, code)
		}
	}
}

// checkNode checks that n lies in its container's span and has the pointer ptr.
func checkNode(t *testing.T, p parsed, n *jsonsrc.Node, outer source.Span, ptr string) {
	t.Helper()
	s := n.Span
	if s.File != outer.File || s.Start >= s.End || s.Start < outer.Start || s.End > outer.End || n.Pointer() != ptr {
		t.Fatalf("node %q at %+v in %+v, pointer %q want %q", p.text(s), s, outer, n.Pointer(), ptr)
	}
	for i, e := range n.Elems {
		checkNode(t, p, e, s, ptr+"/"+strconv.Itoa(i))
	}
	for _, m := range n.Members {
		key := strings.NewReplacer("~", "~0", "/", "~1").Replace(m.Key)
		if k := m.KeySpan; k.Start < s.Start || k.End > m.Value.Span.Start || p.text(k)[0] != '"' {
			t.Fatalf("key %q at %+v", m.Key, k)
		}
		checkNode(t, p, m.Value, s, ptr+"/"+key)
	}
}

// IMPLEMENTATION-PLAN.md §7.7, FORMATTER.md §14.1: Format keeps the tree and is idempotent.
func FuzzFormat(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		p := parse(t, string(data))
		if p.err != nil {
			return
		}
		out := jsonsrc.Format(p.root)
		again := parse(t, string(out))
		if again.err != nil || !sameTree(p.root, again.root) || !bytes.Equal(jsonsrc.Format(again.root), out) {
			t.Fatalf("Format of %q is not a fixed point: %q (%v)", data, out, again.err)
		}
	})
}
