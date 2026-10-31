package gogen_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

const (
	runtimeText = "runtime/rt.go.txt"
	specGlob    = "../../../spec/*.md"
)

// rtName is an `rt.<Name>` of the specification; a name followed by `*` names a family, not one function.
var rtName = regexp.MustCompile(`\brt\.([A-Z][A-Za-z0-9]*)(\*?)`)

// runtimeExports is the exported package-level names of the embedded Go runtime.
func runtimeExports(t *testing.T) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), runtimeText, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() {
				out[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				addExported(out, s)
			}
		}
	}
	return out
}

// addExported adds the exported names a type, const or var spec declares.
func addExported(out map[string]bool, s ast.Spec) {
	switch s := s.(type) {
	case *ast.TypeSpec:
		if s.Name.IsExported() {
			out[s.Name.Name] = true
		}
	case *ast.ValueSpec:
		for _, n := range s.Names {
			if n.IsExported() {
				out[n.Name] = true
			}
		}
	}
}

// CODEGEN.md §6.3 (DECISIONS 415): every `rt.<Name>` the specification names is exported by the embedded runtime.
func TestRuntimeContractNames(t *testing.T) {
	exports := runtimeExports(t)
	docs, err := filepath.Glob(specGlob)
	if err != nil || len(docs) == 0 {
		t.Fatalf("no specification documents at %s: %v", specGlob, err)
	}
	checked := 0
	for _, doc := range docs {
		text, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range rtName.FindAllSubmatch(text, -1) {
			if len(m[2]) > 0 {
				continue
			}
			checked++
			if !exports[string(m[1])] {
				t.Errorf("%s names rt.%s, which %s does not export", filepath.Base(doc), m[1], runtimeText)
			}
		}
	}
	if checked == 0 {
		t.Error("the specification names no rt.<Name>: the scan is wrong")
	}
}
