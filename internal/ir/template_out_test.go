package ir_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
)

// templatesProject gives a root a template for every target items.vocab emits (GRAMMAR.md §7.1).
const templatesProject = `project acme {
  canon: "0.1"
  roots {
    admin: "admin"
    web: "web"
  }
  go_module {
    admin: "example.com/admin"
  }
  out_templates {
    go: { admin: "internal/gen/{dir}" }
    cpp: { admin: "src/{dir}" }
    ts: { web: "src/lib/{dotted}.ts" }
    json: { admin: "data/{dir}/{dotted}.json" }
    view: { web: "views/{dotted}.view.json" }
  }
}
`

// templatedVocab is items.vocab with its emits' out written as root names.
const templatedVocab = `/// Item words.
package items.vocab

/// A colour.
enum Tone { red, blue }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
  /// Its size.
  size: Int = 2
}

/// The badge.
let badge: Badge = { tone: blue }

emit go { out: admin }

emit cpp { out: [admin] }

emit ts { out: web }

emit json { out: admin }

emit view { out: web }
`

// literalVocab is templatedVocab with each root name replaced by the path its template expands to.
var literalVocab = strings.NewReplacer(
	"emit go { out: admin }", `emit go { out: "@admin/internal/gen/items/vocab" }`,
	"emit cpp { out: [admin] }", `emit cpp { out: ["@admin/src/items/vocab"] }`,
	"emit ts { out: web }", `emit ts { out: "@web/src/lib/items.vocab.ts" }`,
	"emit json { out: admin }", `emit json { out: "@admin/data/items/vocab/items.vocab.json" }`,
	"emit view { out: web }", `emit view { out: "@web/views/items.vocab.view.json" }`,
).Replace(templatedVocab)

// templatedTargets is the number of targets items.vocab emits.
const templatedTargets = 5

// emitPlace is where an emit writes, and the names its out gives it.
type emitPlace struct {
	target                                             ir.Target
	out, dir, fileName, goImport, goPackage, namespace string
}

func placement(e *ir.Emit) emitPlace {
	return emitPlace{e.Target, e.Out, e.Dir, e.FileName, e.GoImport, e.GoPackage, e.Namespace}
}

// templateEmits are the emits stage E builds for items.vocab written as src.
func templateEmits(t *testing.T, src string) []*ir.Emit {
	t.Helper()
	w := newWorld(t)
	w.useProject(t, []byte(templatesProject))
	w.add(t, "items/vocab/vocab.canon", []byte(src))
	pkgs := w.build(t)
	if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
		t.Fatalf("findings:\n%s", out)
	}
	if len(pkgs) != 1 {
		t.Fatalf("%d packages built", len(pkgs))
	}
	return pkgs[0].Emits
}

// CODEGEN.md §2.1 "Templates", GRAMMAR.md §7.1, DECISIONS 215, 439: a root name is placed exactly as its expansion.
func TestTemplateOutMatchesLiteral(t *testing.T) {
	templated, literal := templateEmits(t, templatedVocab), templateEmits(t, literalVocab)
	if len(templated) != len(literal) || len(literal) != templatedTargets {
		t.Fatalf("%d emits from templates, %d from literals", len(templated), len(literal))
	}
	for i, got := range templated {
		if g, w := placement(got), placement(literal[i]); g != w {
			t.Errorf("emit %d: from a template %+v, from its expansion %+v", i, g, w)
		}
		if got.Dir == "" {
			t.Errorf("emit %d (%s) is not placed", i, got.Out)
		}
	}
}

// CODEGEN.md §2.1 "Templates", §2.4, DECISIONS 439 (a pin): root names build the bytes their expansions build.
func TestTemplateBuildMatchesLiteral(t *testing.T) {
	templated, literal := templateBuild(t, templatedVocab), templateBuild(t, literalVocab)
	if len(templated) == 0 || !slices.EqualFunc(templated, literal, func(a, b build.Output) bool {
		return a.Path == b.Path && bytes.Equal(a.Content, b.Content)
	}) {
		t.Errorf("outputs differ: %d from templates, %d from literals", len(templated), len(literal))
	}
}

// templateBuild builds templatesProject with items.vocab written as src, in a new directory.
func templateBuild(t *testing.T, src string) []build.Output {
	t.Helper()
	dir := t.TempDir()
	for _, f := range [][2]string{{projectFile, templatesProject}, {"items/vocab/vocab.canon", src}} {
		name, data := f[0], f[1]
		at := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Errors != 0 {
		t.Fatalf("%d build errors", res.Summary.Errors)
	}
	return res.Outputs
}
