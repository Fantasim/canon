package canon_test

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// exampleFiles are the committed example files whose name ends with ext, in byte order.
func exampleFiles(ext string) []string {
	var names []string
	for _, name := range slices.Sorted(maps.Keys(committedExamples())) {
		if strings.HasSuffix(name, ext) && !strings.Contains(name, "/expected/") {
			names = append(names, name)
		}
	}
	return names
}

// API.md T1: every .canon file of the examples, project.canon included, formats to a fixed
// point: Format(Format(x)) == Format(x).
func TestFormatIsIdempotent(t *testing.T) {
	names := exampleFiles(".canon")
	if len(names) == 0 {
		t.Fatal("no example sources")
	}
	for _, name := range names {
		once, err := canon.Format(name, committedExamples()[name])
		if err != nil {
			t.Errorf("API.md T1 %s: %v", name, err)
			continue
		}
		if twice, err := canon.Format(name, once); err != nil || !bytes.Equal(twice, once) {
			t.Errorf("API.md T1 %s: Format(Format(x)) != Format(x) (%v)", name, err)
		}
	}
}

// API.md T1: the canonical layout, CRLF read as LF; project.canon and project.local.canon are
// formatted as project files.
func TestFormatLayout(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"a/a.canon", "package a\r\nconst N=7\r\n", "package a\n\nconst N = 7\n"},
		{"law/project.canon", "project demo {\n  canon:\"0.1\"\n}\n", "project demo {\n  canon: \"0.1\"\n}\n"},
		{"law/project.local.canon", "project demo {\n  roots: {\n    a:\"/a\"\n  }\n}\n", "project demo {\n  roots {\n    a: \"/a\"\n  }\n}\n"},
	} {
		got, err := canon.Format(c.name, []byte(c.src))
		if err != nil || string(got) != c.want {
			t.Errorf("API.md T1 %s: %q, %v, want %q", c.name, got, err, c.want)
		}
	}
}

// API.md T1, API.md X1: a file that does not parse is a *SyntaxError wrapping ErrSyntax, with its
// findings located in the file; a project file under a source's name does not parse either.
func TestFormatSyntaxError(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"a/a.canon", "package a\n\nlet = 1\n"},
		{"a/a.canon", "project demo {\n  canon: \"0.1\"\n}\n"},
	} {
		_, err := canon.Format(c.name, []byte(c.src))
		var se *canon.SyntaxError
		if !errors.As(err, &se) || !errors.Is(err, canon.ErrSyntax) || len(se.Findings) == 0 || se.Findings[0].File != c.name {
			t.Errorf("API.md T1 %q: %v", c.src, err)
			continue
		}
		f := se.Findings[0]
		if want := "syntax error: " + c.name; !strings.HasPrefix(err.Error(), want) || f.Severity != canon.SeverityError {
			t.Errorf("API.md X1: %q, want it to start with %q", err, want)
		}
	}
}

// API.md T2: every JSON source of the examples formats to a fixed point, keys in their order.
func TestFormatJSONSourceIsIdempotent(t *testing.T) {
	names := exampleFiles(".json")
	if len(names) == 0 {
		t.Fatal("no example JSON sources")
	}
	for _, name := range names {
		once, err := canon.FormatJSONSource(committedExamples()[name])
		if err != nil {
			t.Errorf("API.md T2 %s: %v", name, err)
			continue
		}
		if twice, err := canon.FormatJSONSource(once); err != nil || !bytes.Equal(twice, once) {
			t.Errorf("API.md T2 %s: not a fixed point (%v)", name, err)
		}
	}
	got, err := canon.FormatJSONSource([]byte("{\"z\":1,\"a\":{\"y\":2,\"b\":3}}"))
	if err != nil || strings.Index(string(got), `"z"`) > strings.Index(string(got), `"a"`) ||
		strings.Index(string(got), `"y"`) > strings.Index(string(got), `"b"`) {
		t.Errorf("API.md T2: key order not kept: %s %v", got, err)
	}
}

// API.md T2: input that is not JSON, repeats a key or is not UTF-8 (WIRE.md 3) is a
// *SyntaxError holding the finding that says why.
func TestFormatJSONSourceSyntaxError(t *testing.T) {
	for _, c := range []struct {
		src  string
		code diag.Code
	}{
		{"{\"a\": }", diag.E7109.Def().Code},
		{"[1, 2", diag.E7109.Def().Code},
		{"{\"a\": 1, \"a\": 2}", diag.E7104.Def().Code},
		{"[\"\xff\"]", diag.E7105.Def().Code},
	} {
		_, err := canon.FormatJSONSource([]byte(c.src))
		var se *canon.SyntaxError
		if !errors.As(err, &se) || !errors.Is(err, canon.ErrSyntax) || len(se.Findings) == 0 || se.Findings[0].Code != string(c.code) {
			t.Errorf("API.md T2 %q: %v, want %s", c.src, err, c.code)
		}
	}
}
