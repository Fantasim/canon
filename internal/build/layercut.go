package build

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// cutKey orders one entry for F7's cut: its package, then F2's order, then the rest of its key.
type cutKey struct {
	pkg string
	findingKey
	sev   diag.Severity
	layer string
	at    int
}

// cut is each part's kept findings and the counts: every package's findings in F2 order, the first
// limit kept (own for the project's, package "") and the rest counted in Truncated, package-name
// order, as the bags cut them (API.md F7, F2; DECISIONS 463).
func (m *merge) cut(limit, own int) ([][]diag.Finding, diag.Summary) {
	keys := make([]cutKey, len(m.entries))
	for i, e := range m.entries {
		keys[i] = cutKey{pkg: e.f.Package, findingKey: findingKeyOf(m.parts[e.part].Files(), e.f), sev: e.f.Severity, layer: e.f.Layer, at: i}
	}
	slices.SortStableFunc(keys, compareCut)
	kept := make([][]diag.Finding, len(m.parts))
	var sum diag.Summary
	n := 0
	for i, k := range keys {
		if i == 0 || k.pkg != keys[i-1].pkg {
			n = 0
		}
		e := m.entries[k.at]
		tally(&sum.Errors, &sum.Warnings, k.sev)
		if n < limit && k.pkg != "" || n < own && k.pkg == "" {
			kept[e.part] = append(kept[e.part], e.f)
			n++
			continue
		}
		dropped(&sum, k)
	}
	return kept, sum
}

// dropped counts k in its package's Truncation, the last of sum's since keys come by package.
func dropped(sum *diag.Summary, k cutKey) {
	if last := len(sum.Truncated) - 1; last < 0 || sum.Truncated[last].Package != k.pkg {
		sum.Truncated = append(sum.Truncated, diag.Truncation{Package: k.pkg})
	}
	t := &sum.Truncated[len(sum.Truncated)-1]
	tally(&t.Errors, &t.Warnings, k.sev)
}

// tally counts an error or a warning of severity sev; a runtime finding counts as neither.
func tally(errs, warnings *int, sev diag.Severity) {
	switch sev {
	case diag.Error:
		*errs++
	case diag.Warning:
		*warnings++
	case diag.Runtime:
	}
}

func compareCut(a, b cutKey) int {
	return cmp.Or(
		cmp.Compare(a.pkg, b.pkg),
		cmp.Compare(a.file, b.file), cmp.Compare(a.line, b.line), cmp.Compare(a.col, b.col),
		cmp.Compare(a.code, b.code), cmp.Compare(a.message, b.message),
		cmp.Compare(a.sev, b.sev), cmp.Compare(a.endLine, b.endLine), cmp.Compare(a.endCol, b.endCol),
		cmp.Compare(a.path, b.path), cmp.Compare(a.layer, b.layer),
	)
}

// shifted is f with every span's file id moved by from; no file stays no file.
func shifted(f diag.Finding, from source.FileID) diag.Finding {
	f.Span = shift(f.Span, from)
	f.Related = slices.Clone(f.Related)
	for i := range f.Related {
		f.Related[i].Span = shift(f.Related[i].Span, from)
	}
	f.Stack = slices.Clone(f.Stack)
	for i := range f.Stack {
		f.Stack[i].Span = shift(f.Stack[i].Span, from)
	}
	return f
}

func shift(sp source.Span, from source.FileID) source.Span {
	if sp.File != source.NoFile {
		sp.File += from
	}
	return sp
}

// filePart is a file set whose ids are placed past from.
type filePart struct {
	from  source.FileID
	files diag.Files
}

// mergedFiles resolves a merged id in the part it came from, the latest part first.
type mergedFiles []filePart

func (m mergedFiles) at(id source.FileID) (diag.Files, source.FileID) {
	for i := len(m) - 1; i > 0; i-- {
		if id > m[i].from {
			return m[i].files, id - m[i].from
		}
	}
	return m[0].files, id
}

func (m mergedFiles) Path(id source.FileID) string {
	files, local := m.at(id)
	return files.Path(local)
}

func (m mergedFiles) Position(id source.FileID, p source.Pos) (line, col int) {
	files, local := m.at(id)
	return files.Position(local, p)
}

func (m mergedFiles) Content(id source.FileID) []byte {
	files, local := m.at(id)
	return files.Content(local)
}
