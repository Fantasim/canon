package specfence_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/specfence"
)

// The reader of the spec tests of the runtimes (CODEGEN.md §7.4, §8.2): the first block of the language after the heading, nothing else.
func TestFind(t *testing.T) {
	t.Parallel()
	const doc = "```ts\nearly\n```\n## H\n\n```cpp\nint x;\n```\n\n```ts\nlate\n```\n"
	for _, c := range []struct {
		name, heading, lang, want string
		err                       error
	}{
		{name: "first block after", heading: "## H", lang: "cpp", want: "int x;\n"},
		{name: "not the one before the heading", heading: "## H", lang: "ts", want: "late\n"},
		{name: "no heading", heading: "## G", lang: "ts", err: specfence.ErrNoHeading},
		{name: "no block", heading: "## H", lang: "go", err: specfence.ErrNoBlock},
	} {
		got, err := specfence.Find(doc, c.heading, c.lang)
		if !errors.Is(err, c.err) || got != c.want {
			t.Errorf("%s: Find = %q, %v; want %q, %v", c.name, got, err, c.want, c.err)
		}
	}
	if _, err := specfence.Find("## H\n```ts\nopen\n", "## H", "ts"); !errors.Is(err, specfence.ErrUnclosed) {
		t.Errorf("an unclosed block: %v", err)
	}
}

// Block reads the file the path names.
func TestBlockReadsTheFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(path, []byte("## H\n```ts\nx\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := specfence.Block(t, path, "## H", "ts"); got != "x\n" {
		t.Errorf("Block = %q", got)
	}
}
