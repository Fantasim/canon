package gorules

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// boundary is one imports.tsv row: packages under from must not import forbid.
type boundary struct {
	from, forbid string
	line         int
}

// loadBoundaries parses root's .sovaudit/imports.tsv; a missing file is no boundaries, a
// bad row is skipped and reported.
func loadBoundaries(root string) ([]boundary, []lane.Skip, error) {
	data, err := fs.ReadFile(os.DirFS(root), repo.ImportsFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errImports, err)
	}
	var out []boundary
	var skips []lane.Skip
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, commentMark) {
			continue
		}
		cols := strings.Split(l, fieldSep)
		if len(cols) != pairLen || !validPattern(cols[firstArg]) || !validPattern(cols[secondArg]) {
			skips = append(skips, lane.Skip{What: ruleImportBound, Reason: fmt.Sprintf(skipBoundaryRow, repo.ImportsFile, n)})
			continue
		}
		out = append(out, boundary{from: strings.TrimSpace(cols[firstArg]), forbid: strings.TrimSpace(cols[secondArg]), line: n})
	}
	if err := sc.Err(); err != nil {
		return out, skips, fmt.Errorf("%w: %w", errImports, err)
	}
	return out, skips, nil
}

func validPattern(p string) bool {
	_, err := path.Match(strings.TrimSuffix(strings.TrimSpace(p), boundaryAny), "")
	return err == nil && strings.TrimSpace(p) != ""
}

// matchTree is path.Match, where a trailing "/..." also matches every path below.
func matchTree(pattern, p string) bool {
	if base, ok := strings.CutSuffix(pattern, boundaryAny); ok {
		for q := p; ; q = path.Dir(q) {
			if m, _ := path.Match(base, q); m {
				return true
			}
			if q == dirHere || q == pathSep || !strings.Contains(q, pathSep) {
				return false
			}
		}
	}
	m, _ := path.Match(pattern, p)
	return m
}

// importBoundary reports each import a boundary row forbids for the importing package.
func (s *scan) importBoundary() []lane.Skip {
	rows, skips, err := loadBoundaries(s.ctx.Repo.Root)
	if err != nil {
		return append(skips, lane.Skip{What: ruleImportBound, Reason: err.Error()})
	}
	for _, f := range s.files {
		for _, is := range f.AST.Imports {
			p, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				continue
			}
			if b, ok := s.forbidden(rows, f.Dir, p); ok {
				s.emit(finding.Finding{
					Rule: ruleImportBound, File: f.Path, Line: s.line(is.Pos()), Detail: p,
					Message: fmt.Sprintf(msgBoundary, f.Dir, b.forbid, repo.ImportsFile, b.line), Fix: fixBoundary,
				})
			}
		}
	}
	return skips
}

// forbidden finds the first row whose from-pattern covers dir and whose forbidden pattern
// covers the import, given as a full path or relative to this module.
func (s *scan) forbidden(rows []boundary, dir, imp string) (boundary, bool) {
	rel, inModule := strings.CutPrefix(imp, s.ctx.Repo.Module+pathSep)
	for _, b := range rows {
		if !matchTree(b.from, dir) {
			continue
		}
		if matchTree(b.forbid, imp) || inModule && matchTree(b.forbid, rel) {
			return b, true
		}
	}
	return boundary{}, false
}
