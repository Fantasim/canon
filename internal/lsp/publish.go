package lsp

import (
	"cmp"
	"maps"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

type diagnostic struct {
	Range              textRange     `json:"range"`
	Severity           int           `json:"severity"`
	Code               string        `json:"code"`
	Source             string        `json:"source"`
	Message            string        `json:"message"`
	RelatedInformation []relatedInfo `json:"relatedInformation,omitempty"`
}

type relatedInfo struct {
	Location location `json:"location"`
	Message  string   `json:"message"`
}

type location struct {
	URI   string    `json:"uri"`
	Range textRange `json:"range"`
}

type publishParams struct {
	URI         string       `json:"uri"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

// fileSet locates a finding's file by id: the *source.FileSet of a build, or of a loose parse.
type fileSet interface {
	File(id source.FileID) *source.File
}

// converter turns findings into diagnostics, indexing each file's lines once.
type converter struct {
	set   fileSet
	lines map[source.FileID]*lines
	root  string
}

func newConverter(set fileSet, root string) *converter {
	return &converter{set: set, lines: map[source.FileID]*lines{}, root: root}
}

// diagnosticsOf is f's findings as diagnostics by absolute file name, each file's in F2 order;
// a finding with no file goes to project.canon of root.
func diagnosticsOf(f build.Findings, root string) (map[string][]diagnostic, error) {
	out := map[string][]diagnostic{}
	if len(f.List) == 0 {
		return out, nil
	}
	set, ok := f.Files.(fileSet)
	if !ok {
		return nil, errFileSet
	}
	c := newConverter(set, root)
	located := diag.Locate(f.Files, f.List)
	for i, raw := range f.List {
		abs, d := c.diagnostic(raw, located[i])
		out[abs] = append(out[abs], d)
	}
	//canon:unordered each file's list is sorted on its own
	for _, list := range out {
		sortDiagnostics(list)
	}
	return out, nil
}

func (c *converter) diagnostic(raw diag.Finding, l diag.Located) (string, diagnostic) {
	d := diagnostic{Severity: severityError, Code: string(l.Code), Source: canonName, Message: l.Message}
	if l.Severity == diag.Warning {
		d.Severity = severityWarning
	}
	abs, rng, ok := c.place(raw.Span.File, l.Loc)
	if !ok {
		abs = project.Join(c.root, project.FileName)
	}
	d.Range = rng
	for j, r := range raw.Related {
		rabs, rrng, ok := c.place(r.Span.File, l.Related[j].Loc)
		if ok {
			d.RelatedInformation = append(d.RelatedInformation, relatedInfo{Location: location{URI: uriOf(rabs), Range: rrng}, Message: l.Related[j].Note})
		}
	}
	return abs, d
}

// place is the absolute name and UTF-16 range of a location in file id, false for no file.
func (c *converter) place(id source.FileID, loc source.Location) (string, textRange, bool) {
	file := c.set.File(id)
	if file == nil {
		return "", textRange{}, false
	}
	l := c.lines[id]
	if l == nil {
		l = newLines(file.Content)
		c.lines[id] = l
	}
	return file.Abs, l.span(loc), true
}

// spanOf is the absolute name and UTF-16 range of a span, false for a span in no file.
func (c *converter) spanOf(at source.Span) (string, textRange, bool) {
	file := c.set.File(at.File)
	if file == nil {
		return "", textRange{}, false
	}
	var loc source.Location
	loc.Line, loc.Col = file.Position(at.Start)
	loc.EndLine, loc.EndCol = file.Position(at.End)
	return c.place(at.File, loc)
}

// sortDiagnostics orders one file's diagnostics as API.md F2 orders findings.
func sortDiagnostics(list []diagnostic) {
	slices.SortStableFunc(list, func(a, b diagnostic) int {
		return cmp.Or(cmp.Compare(a.Range.Start.Line, b.Range.Start.Line), cmp.Compare(a.Range.Start.Character, b.Range.Start.Character),
			cmp.Compare(a.Code, b.Code), cmp.Compare(a.Message, b.Message))
	})
}

// looseDiagnostics is the syntax findings of a .canon buffer in no project; nothing for a
// closed one or another kind of file.
func looseDiagnostics(lw looseWork) (map[string][]diagnostic, error) {
	if !lw.open || path.Ext(lw.abs) != project.SourceExt {
		return map[string][]diagnostic{}, nil
	}
	set := &source.FileSet{}
	f, err := set.Add(lw.abs, lw.abs, lw.text)
	if err != nil {
		return map[string][]diagnostic{}, err
	}
	bag := diag.NewBag(set, "")
	syntax.Parse(f, fileKind(lw.abs), bag)
	return diagnosticsOf(build.Findings{Files: set, List: bag.Findings()}, project.DirOf(lw.abs))
}

// publish replaces each owner's diagnostics with its new lists, then sends, in name order,
// every file a recomputed owner had or has diagnostics in, changed or not: its findings from
// every owner, or an empty list.
func (s *server) publish(lists map[string]map[string][]diagnostic, uris map[string]string) {
	changed := map[string]bool{}
	for _, owner := range slices.Sorted(maps.Keys(lists)) {
		s.replace(owner, lists[owner], changed)
	}
	for _, abs := range slices.Sorted(maps.Keys(changed)) {
		uri, open := uris[abs]
		if !open {
			uri = uriOf(abs)
		}
		_ = s.notify(methodPublish, publishParams{URI: uri, Diagnostics: merged(s.published[abs])}) // a failed write sticks: loop returns it
		if len(s.published[abs]) == 0 {
			delete(s.published, abs)
		}
	}
}

// replace makes files the owner's only diagnostics: its lists in files it no longer has any
// in are dropped. Every file it had or has a list in is marked in changed, to be sent again.
func (s *server) replace(owner string, files map[string][]diagnostic, changed map[string]bool) {
	//canon:unordered every file is marked, sent in order by publish
	for abs, owners := range s.published {
		if _, ok := owners[owner]; ok {
			delete(owners, owner)
			changed[abs] = true
		}
	}
	//canon:unordered every file is marked, sent in order by publish
	for abs, list := range files {
		if s.published[abs] == nil {
			s.published[abs] = map[string][]diagnostic{}
		}
		s.published[abs][owner] = list
		changed[abs] = true
	}
}

// merged is a file's diagnostics from all its owners, in F2 order; empty, never nil.
func merged(owners map[string][]diagnostic) []diagnostic {
	out := []diagnostic{}
	for _, owner := range slices.Sorted(maps.Keys(owners)) {
		out = append(out, owners[owner]...)
	}
	if len(owners) > 1 {
		sortDiagnostics(out)
	}
	return out
}
