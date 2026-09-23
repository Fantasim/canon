package diag_test

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const errorsMD = "../../spec/ERRORS.md"

const (
	codesHeader    = "| Code | Severity | Package | Owner | Meaning |"
	messagesHeader = "| Code | Variant | Args | Template |"
	kindsHeader    = "| Kind | Word | Used by |"
	notesHeader    = "| Note | Args | Template |"
)

// M0 acceptance: ERRORS.md regenerated from the registry is unchanged. Message rows are rebuilt
// whole, codes and kinds rows on the columns the registry holds, and the count sentence.
func TestErrorsMDRegeneratedFromRegistry(t *testing.T) {
	doc, err := os.ReadFile(errorsMD)
	if err != nil {
		t.Fatal(err)
	}
	rows := tableRows(string(doc))
	messages, codes := registryRows()
	sameRows(t, "message", rows[messagesHeader], messages, false)
	sameRows(t, "codes", rows[codesHeader], codes, true)
	sameRows(t, "kinds", rows[kindsHeader], kindRows(), true)
	if !slices.Contains(strings.Split(string(doc), "\n"), countSentence()) {
		t.Errorf("ERRORS.md does not say: %s", countSentence())
	}
}

// tableRows groups the rows of every table under its header line, in document order.
func tableRows(doc string) map[string][]string {
	out := map[string][]string{}
	header := ""
	for _, line := range strings.Split(doc, "\n") {
		switch {
		case !strings.HasPrefix(line, "|"):
			header = ""
		case header == "":
			header = line
		case !strings.HasPrefix(line, "|---"):
			out[header] = append(out[header], line)
		}
	}
	return out
}

// registryRows renders the message rows and the codes-row prefixes, in the document's
// order: by number, then E before W.
func registryRows() (messages, codes []string) {
	defs := slices.Clone(diag.Registry)
	slices.SortFunc(defs, func(a, b diag.Def) int {
		return cmp.Or(cmp.Compare(a.Code[1:], b.Code[1:]), cmp.Compare(a.Code, b.Code))
	})
	for _, d := range defs {
		codes = append(codes, fmt.Sprintf("| %s | %s | %s |", d.Code, d.Severity, d.Package))
		for _, v := range d.Variants {
			messages = append(messages, fmt.Sprintf("| %s | %s | %s | `%s` |", d.Code, orDash(v.Name), orDash(args(v.Args)), v.Template))
		}
	}
	return messages, codes
}

// ERRORS.md §1.5: each note renders its template; checkUnnamed is NoteCheck("").
func TestNotesFollowErrorsMD(t *testing.T) {
	doc, err := os.ReadFile(errorsMD)
	if err != nil {
		t.Fatal(err)
	}
	rows := tableRows(string(doc))[notesHeader]
	files := diag.MemFiles{{Path: "a.canon", Content: "record A {\n  productionItem:\tref items\n}\n"}}
	decl := source.Span{File: 1, Start: 13, End: 38}
	notes := map[string]diag.Note{"source": diag.NoteSource(decl), "check": diag.NoteCheck("unreachable_levels"), "checkUnnamed": diag.NoteCheck("")}
	rendered := strings.NewReplacer("{decl}", "productionItem: ref items", "{name}", "unreachable_levels")
	if len(rows) != len(notes) {
		t.Fatalf("ERRORS.md §1.5 has %d notes, diag %d", len(rows), len(notes))
	}
	for _, row := range rows {
		cells := strings.Split(row, "|")
		name, tpl := strings.Trim(cells[1], " `"), strings.Split(cells[3], "`")[1]
		bag := diag.NewBag(files, "p")
		diag.E1004.At(decl).Related(decl, notes[name]).Report(bag)
		if got := bag.Findings()[0].Related[0].Note; got != rendered.Replace(tpl) {
			t.Errorf("note %s: got %q, want %q", name, got, rendered.Replace(tpl))
		}
	}
}

func kindRows() []string {
	var out []string
	for k := diag.Kind(0); k.Word() != ""; k++ {
		out = append(out, fmt.Sprintf("| `%s` | %s |", k, k.Word()))
	}
	return out
}

func countSentence() string {
	n := map[diag.Severity]int{}
	messages := 0
	for _, d := range diag.Registry {
		n[d.Severity]++
		messages += len(d.Variants)
	}
	return fmt.Sprintf("The catalogue holds %d codes: %d errors, %d warnings and %d run-time codes, with %d messages.",
		len(diag.Registry), n[diag.Error], n[diag.Warning], n[diag.Runtime], messages)
}

func args(as []diag.Arg) string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		parts = append(parts, a.Name+":"+a.Type.String())
	}
	return strings.Join(parts, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// sameRows compares the document's rows with the regenerated ones; with prefix, a
// regenerated row is the start of its document row, up to a cell boundary.
func sameRows(t *testing.T, what string, doc, regenerated []string, prefix bool) {
	t.Helper()
	if len(doc) != len(regenerated) {
		t.Errorf("%s rows: ERRORS.md has %d, the registry %d", what, len(doc), len(regenerated))
	}
	for i := range min(len(doc), len(regenerated)) {
		same := doc[i] == regenerated[i]
		if prefix {
			same = strings.HasPrefix(doc[i], regenerated[i]+" ")
		}
		if !same {
			t.Fatalf("%s row %d:\nERRORS.md: %s\nregistry:  %s", what, i, doc[i], regenerated[i])
		}
	}
}
