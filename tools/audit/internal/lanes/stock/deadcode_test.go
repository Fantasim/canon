package stock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

const deadcodeSample = `internal/db/dialect.go:22:6: unreachable func: DialectFor
internal/e2e/client.go:292:18: unreachable func: Client.Post
not a deadcode line, ignore it
`

func TestParseDeadcode(t *testing.T) {
	entries := parseDeadcode(deadcodeSample)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].file != "internal/db/dialect.go" || entries[0].line != 22 || entries[0].name != "DialectFor" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[1].name != "Client.Post" {
		t.Errorf("entry 1 name = %q, want Client.Post", entries[1].name)
	}
}

func TestUnreachableFindings(t *testing.T) {
	fs := unreachableFindings(parseDeadcode(deadcodeSample))
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2", len(fs))
	}
	if fs[0].Rule != ruleDeadUnreachable || fs[0].Detail != "DialectFor" {
		t.Errorf("finding 0 = %+v", fs[0])
	}
	if fs[0].Message != unreachableFuncPfx+"DialectFor" {
		t.Errorf("finding 0 message = %q", fs[0].Message)
	}
}

// writeGoFile creates a .go file under dir/rel with body, parents included.
func writeGoFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), testFilePerm); err != nil {
		t.Fatal(err)
	}
}

func TestDeadFileFindings(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "allDead.go", "package p\n\nfunc A() {}\nfunc B() {}\n")
	writeGoFile(t, dir, "oneLive.go", "package p\n\nfunc C() {}\nfunc D() {}\n")
	writeGoFile(t, dir, "hasType.go", "package p\n\ntype T struct{}\nfunc E() {}\n")
	writeGoFile(t, dir, "typeOnly.go", "package p\n\ntype U struct{}\n")
	writeGoFile(t, dir, "z_test.go", "package p\n\nfunc F() {}\n")

	rel := []string{"allDead.go", "oneLive.go", "hasType.go", "typeOnly.go", "z_test.go"}
	tree, errs := gosrc.Parse(dir, rel)
	if len(errs) != 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	entries := []deadEntry{
		{file: "allDead.go", line: 3, name: "A"},
		{file: "allDead.go", line: 4, name: "B"},
		{file: "oneLive.go", line: 4, name: "D"}, // C is reachable
		{file: "hasType.go", line: 4, name: "E"},
		{file: "z_test.go", line: 3, name: "F"},
	}

	fs := deadFileFindings(tree, entries)
	if len(fs) != 1 {
		t.Fatalf("got %d dead-file findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].File != "allDead.go" {
		t.Errorf("dead file = %q, want allDead.go", fs[0].File)
	}
}
