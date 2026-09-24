package grammar_test

import (
	"bytes"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
	"github.com/fantasim/canonlang/internal/testkit/progen/grammar"
)

const seeds = 200

// DECISIONS 200, GRAMMAR.md §5: a generated file parses clean, the same for the same seed.
func TestGenerateParses(t *testing.T) {
	for seed := range uint64(seeds) {
		src := grammar.Generate(progen.NewRand(seed), progen.NewBudget(120, 6))
		if again := grammar.Generate(progen.NewRand(seed), progen.NewBudget(120, 6)); !bytes.Equal(src, again) {
			t.Fatalf("seed %d: two different files", seed)
		}
		if _, findings := grammar.Parse("gen/gen.canon", src); len(findings) > 0 {
			t.Errorf("seed %d: %s %s\n%s", seed, findings[0].Code, findings[0].Message, src)
		}
	}
}

// FORMATTER.md §1, §9.1, §10: the shape sees no layout, only meaning.
func TestShape(t *testing.T) {
	shape := func(src string) string {
		tree, findings := grammar.Parse("gen/gen.canon", []byte(src))
		if len(findings) > 0 {
			t.Fatalf("%q: %s", src, findings[0].Message)
		}
		return grammar.Shape(tree)
	}
	base := shape("package a\nimport z\nimport b { Y, X }\nlet d = [60s, 1]\n")
	same := shape("package a\n\nimport b { X, Y }\nimport z\n\nlet d = [\n  1m,\n  1,\n]\n")
	other := shape("package a\nimport z\nimport b { Y, X }\nlet d = [61s, 1]\n")
	if base != same {
		t.Errorf("layout changed the shape:\n%s\n---\n%s", base, same)
	}
	if base == other {
		t.Error("a different duration kept the shape")
	}
}

// FORMATTER.md §8, §10, §11: comments are kept, moved only with their import, a doc line spaced.
func TestShapeComments(t *testing.T) {
	shape := func(src string) string {
		tree, _ := grammar.Parse("gen/gen.canon", []byte(src))
		return grammar.Shape(tree)
	}
	base := shape("package a\n// c1\nimport z\nimport b\n///c2\nlet d = 1 // c3\n")
	same := shape("package a\nimport b\n// c1\nimport z\n\n/// c2\nlet d = 1 // c3   \n")
	for _, other := range []string{
		"package a\n// c1\nimport z\nimport b\n///c2\nlet d = 1\n",
		"package a\n// c1\nimport z\nimport b\n///c2\nlet d = 1 // c4\n",
		"package a\nimport z\nimport b\n///c2\n// c1\nlet d = 1 // c3\n",
		"package a\n// c1\nimport z\nimport b\n///c2\nlet d = 1\n// c3\n",
		"package a\n// c1\nimport z\nimport b\n///c2\n\nlet d = 1 // c3\n",
	} {
		if shape(other) == base {
			t.Errorf("%q kept the shape of the comments", other)
		}
	}
	if base != same {
		t.Errorf("import order or doc spacing changed the shape:\n%s\n---\n%s", base, same)
	}
	if shape("package a\nlet d = 1 // c\nlet e = 2\n") == shape("package a\nlet d = 1\n// c\nlet e = 2\n") {
		t.Error("a trailing comment and an own-line one have one shape")
	}
	if shape("package a\nlet d = 1 /* c\n */\nlet e = 2\n") != shape("package a\nlet d = 1\n/* c\n */\nlet e = 2\n") {
		t.Error("a block comment over two lines is an own-line comment wherever it starts")
	}
}

// DECISIONS 200: a corruption changes the file, deterministically from its seed.
func TestCorrupt(t *testing.T) {
	src := grammar.Generate(progen.NewRand(3), progen.NewBudget(80, 5))
	changed := 0
	for seed := range uint64(seeds) {
		bad := grammar.Corrupt(progen.NewRand(seed), src)
		if !bytes.Equal(bad, grammar.Corrupt(progen.NewRand(seed), src)) {
			t.Fatalf("seed %d: two different corruptions", seed)
		}
		if !bytes.Equal(bad, src) {
			changed++
		}
	}
	if changed < seeds/2 {
		t.Errorf("only %d of %d corruptions changed the file", changed, seeds)
	}
}
