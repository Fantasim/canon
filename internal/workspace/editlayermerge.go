package workspace

import (
	"context"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// merge is the session's re-check with the findings its inactive layers add (API.md E18; DECISIONS
// 463), every list read whole (F7). Each analysis has its own file set: an entry keeps its ids in
// its part's set until the result places each part's ids past the ones before (mergedFiles).
type merge struct {
	parts   []*build.Analysis // the session's re-check first, then one per layer
	entries []merged
	base    map[findingKey]bool // the session's own findings: never repeated
	added   map[layeredKey]bool // what layers added, by the layer it names
}

// merged is one finding of the merge, in the source language, by the part it came from.
type merged struct {
	f    diag.Finding
	part int
}

// findingKey is two findings the same: code, span by display path (one layout, §1.3), path, message.
type findingKey struct {
	code                       diag.Code
	file, path, message        string
	line, col, endLine, endCol int
}

// layeredKey is a finding's key with the layer it names.
type layeredKey struct {
	findingKey
	layer string
}

func newMerge(session *build.Analysis) *merge {
	m := &merge{parts: []*build.Analysis{session}, base: map[findingKey]bool{}, added: map[layeredKey]bool{}}
	for _, f := range session.OwnWhole() {
		m.base[keyOf(session.Files(), f)] = true
		m.entries = append(m.entries, merged{f: f})
	}
	return m
}

// add takes the findings of layer's analysis a that the session's do not have; one that names no
// layer is named by layer, one that names the layer whose amendment made its value keeps it (F11).
func (m *merge) add(layer string, a *build.Analysis) {
	part := len(m.parts)
	m.parts = append(m.parts, a)
	for _, f := range a.OwnWhole() {
		k := keyOf(a.Files(), f)
		if f.Layer == "" {
			f.Layer = layer
		}
		if m.base[k] || m.added[layeredKey{k, f.Layer}] {
			continue
		}
		m.added[layeredKey{k, f.Layer}] = true
		m.entries = append(m.entries, merged{f: f, part: part})
	}
}

// result is the merged findings, each package cut once at the session's limit (F7), in lang; the
// session's own result when no layer added a finding.
func (m *merge) result(ctx context.Context, session *build.Result, lang string) (*build.Result, error) {
	if len(m.added) == 0 {
		return session, nil
	}
	kept, sum := m.cut(m.parts[0].MaxFindings(), diag.DefaultMaxFindings) // the project's own bag keeps diag's default
	sum.Packages = session.Summary.Packages
	out := *session
	out.Findings = build.Findings{Summary: sum}
	var files mergedFiles
	var top source.FileID
	for i, a := range m.parts {
		list, err := a.InOrder(ctx, kept[i], lang)
		if err != nil {
			return session, err
		}
		next := top
		for _, f := range list {
			next = max(next, top+topID(f))
			out.List = append(out.List, shifted(f, top))
		}
		files = append(files, filePart{from: top, files: a.Files()})
		top = next
	}
	out.Files = files
	return &out, nil
}

// keyOf is f's findingKey, its span located in files.
func keyOf(files diag.Files, f diag.Finding) findingKey {
	k := findingKey{code: f.Code, file: files.Path(f.Span.File), path: f.Path, message: f.Message}
	k.line, k.col = files.Position(f.Span.File, f.Span.Start)
	k.endLine, k.endCol = files.Position(f.Span.File, f.Span.End)
	return k
}
