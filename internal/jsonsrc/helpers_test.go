package jsonsrc_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// parsed is the result of reading one text as the file a.json.
type parsed struct {
	fs       *source.FileSet
	file     *source.File
	root     *jsonsrc.Node
	err      error
	findings []diag.Finding
}

func parse(t testing.TB, text string) parsed {
	t.Helper()
	fs := &source.FileSet{}
	f, err := fs.Add("a.json", "/p/a.json", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fs, "p")
	root, err := jsonsrc.Parse(f, bag)
	return parsed{fs: fs, file: f, root: root, err: err, findings: bag.Findings()}
}

// mustParse is parse of a text that is valid JSON.
func mustParse(t testing.TB, text string) parsed {
	t.Helper()
	p := parse(t, text)
	if p.err != nil || len(p.findings) > 0 {
		t.Fatalf("%q: %v, %v", text, p.err, p.findings)
	}
	return p
}

// walk calls fn on n and every node under it, in document order.
func walk(n *jsonsrc.Node, fn func(*jsonsrc.Node)) {
	fn(n)
	for _, e := range n.Elems {
		walk(e, fn)
	}
	for _, m := range n.Members {
		walk(m.Value, fn)
	}
}

// text is the source text a span covers.
func (p parsed) text(s source.Span) string {
	return string(p.file.Content[s.Start:s.End])
}
