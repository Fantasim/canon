package ir_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

const hintDefinesHead = "package a\n\n/// Words; the defines keep the names.\nenum Word @cpp(defines: \"\") {\n"

// hintFindings builds program files with a package `a` whose enum Word defines names, the files read through swap (names and contents), and `import a` added to p when withImport; it returns every finding of the other packages, as code and message, sorted.
func hintFindings(t *testing.T, files map[string]string, names []string, swap *strings.Replacer, withImport bool) []string {
	t.Helper()
	w := newWorld(t)
	var b strings.Builder
	b.WriteString(hintDefinesHead)
	for _, n := range names {
		b.WriteString("  /// Spelled like a generated name.\n  " + n + "\n")
	}
	b.WriteString("}\n\nemit cpp { out: \"@features/a\", namespace: \"a\", mode: types }\n")
	w.add(t, "a/a.canon", []byte(b.String()))
	for _, name := range slices.Sorted(maps.Keys(files)) {
		text := files[name]
		if withImport && name == canonFile {
			text = strings.Replace(text, hintPackage, hintImport, 1)
		}
		if swap != nil {
			name, text = swap.Replace(name), swap.Replace(text)
		}
		w.add(t, name, []byte(text))
	}
	w.build(t)
	var out []string
	for _, f := range w.parse.Findings() {
		out = append(out, string(f.Code)+" "+f.Message)
	}
	for _, pkg := range slices.Sorted(maps.Keys(w.bags)) {
		for _, f := range w.bags[pkg].Findings() {
			if pkg != "a" {
				out = append(out, string(f.Code)+" "+f.Message)
			}
		}
	}
	slices.Sort(out)
	return out
}

// hintNames are the distinct names of prog's rows.
func (p hintProgram) names() []string {
	var out []string
	for _, r := range p.rows {
		if !slices.Contains(out, r.name) {
			out = append(out, r.name)
		}
	}
	return out
}

// named are the rows of p for name, but those of item when except is set, as expected findings.
func (p hintProgram) named(name, except string) []string {
	var hints []foreignHint
	for _, r := range p.rows {
		if r.name == name && (except == "" || r.item != except) {
			hints = append(hints, r.foreignHint)
		}
	}
	return expectedHints(&source.FileSet{}, hints)
}

// TestForeignHintKinds is CODEGEN.md §7.9, ERRORS.md E8005 and DECISIONS 403, 213, 305, 442 (amends 403; 213: a refused name is no define; 305: every ban names a way out that works): a name built on the Canon name of a field, a fn or a parameter is `foreignDerived`, a name the generator fixes is `foreignFixed`, `<Element>Row` names its record, a name met in two ways is a finding for each, and a table id, a field's getter, an enum member and a fn's name keep their variants. Each message is the registry's own.
func TestForeignHintKinds(t *testing.T) {
	for i, prog := range hintPrograms {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			got := hintFindings(t, prog.files, prog.names(), nil, false)
			var hints []foreignHint
			for _, r := range prog.rows {
				hints = append(hints, r.foreignHint)
			}
			if want := expectedHints(&source.FileSet{}, hints); !slices.Equal(got, want) {
				t.Errorf("findings\n%q\nwant\n%q", got, want)
			}
		})
	}
}

// TestForeignHintsClear is CODEGEN.md §7.9 and DECISIONS 403, 213, 305, 442 (305: every ban names a way out that works): doing exactly what a finding says (`@cpp(name:)` on {a}, the Canon rename of {a}, renaming the entry, or `import a { Word }` for a name that no override changes) clears that finding, and leaves the same name's findings for other items alone.
func TestForeignHintsClear(t *testing.T) {
	for i, prog := range hintPrograms {
		for _, row := range prog.rows {
			t.Run(fmt.Sprint(i, row.name, row.item), func(t *testing.T) {
				names := []string{row.name}
				if got, want := hintFindings(t, prog.files, names, nil, false), prog.named(row.name, ""); !slices.Equal(got, want) {
					t.Fatalf("before the way out\n%q\nwant\n%q", got, want)
				}
				var swap *strings.Replacer
				var want []string
				if row.swap != nil {
					swap, want = strings.NewReplacer(row.swap...), prog.named(row.name, row.item)
				}
				if got := hintFindings(t, prog.files, names, swap, row.swap == nil); !slices.Equal(got, want) {
					t.Errorf("after the way out\n%q\nwant\n%q", got, want)
				}
			})
		}
	}
}
