package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// merge is a run's own analysis with what its inactive layers add, every list read whole (API.md
// E18, F7; DECISIONS 463, 469). An entry keeps its ids in its part's own file set until the result
// places each part's set past the ones before (mergedFiles).
type merge struct {
	parts   []*Analysis // the run's own analysis first, then one per layer
	entries []merged
	whole   func(*Analysis) []diag.Finding // an analysis's findings, uncut
	base    map[findingKey]bool            // the run's own findings: never repeated
	added   map[layeredKey]bool            // what layers added, by the layer it names
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

func newMerge(session *Analysis, whole func(*Analysis) []diag.Finding) *merge {
	m := &merge{parts: []*Analysis{session}, whole: whole, base: map[findingKey]bool{}, added: map[layeredKey]bool{}}
	for _, f := range whole(session) {
		m.base[findingKeyOf(session.Files(), f)] = true
		m.entries = append(m.entries, merged{f: f})
	}
	return m
}

// add takes the findings of layer's analysis a that the run's own do not have; one that names no
// layer is named by layer, one that names the layer whose amendment made its value keeps it (F11).
func (m *merge) add(layer string, a *Analysis) {
	part := len(m.parts)
	m.parts = append(m.parts, a)
	for _, f := range m.whole(a) {
		k := findingKeyOf(a.Files(), f)
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

// addedErrors is how many errors the layers add to the run's own findings.
func (m *merge) addedErrors() int {
	n := 0
	for _, e := range m.entries {
		if e.part > 0 && e.f.Severity == diag.Error {
			n++
		}
	}
	return n
}

// result is the merged findings, each package cut once at the run's limit (F7), in lang; the run's
// own result, session, when no layer added a finding. Every id of the run's own file set keeps its
// meaning, so findings of that set may join the result's list.
func (m *merge) result(ctx context.Context, session *Result, lang string) (*Result, error) {
	if len(m.added) == 0 {
		return session, nil
	}
	kept, sum := m.cut(m.parts[0].MaxFindings(), diag.DefaultMaxFindings) // the project's own bag keeps diag's default
	sum.Packages = session.Summary.Packages
	out := *session
	out.Findings = Findings{Summary: sum}
	var files mergedFiles
	var top source.FileID
	for i, a := range m.parts {
		list, err := a.InOrder(ctx, kept[i], lang)
		if err != nil {
			return session, err
		}
		for _, f := range list {
			out.List = append(out.List, shifted(f, top))
		}
		files = append(files, filePart{from: top, files: a.Files()})
		top += lastFile(a.r.s.set)
	}
	out.Files = files
	return &out, nil
}

// lastFile is the id of the last file set holds now.
func lastFile(set *source.FileSet) source.FileID {
	lo, hi := source.NoFile, source.FileID(1) // set holds lo (or lo is none) and not hi
	for set.File(hi) != nil {
		lo, hi = hi, hi+hi
	}
	for lo+1 < hi {
		mid := lo + (hi-lo)>>1
		if set.File(mid) != nil {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// findingKeyOf is f's findingKey, its span located in files.
func findingKeyOf(files diag.Files, f diag.Finding) findingKey {
	k := findingKey{code: f.Code, file: files.Path(f.Span.File), path: f.Path, message: f.Message}
	k.line, k.col = files.Position(f.Span.File, f.Span.Start)
	k.endLine, k.endCol = files.Position(f.Span.File, f.Span.End)
	return k
}
