package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

type formattingParams struct {
	TextDocument documentID `json:"textDocument"`
}

// textEdit is LSP 3.17's TextEdit.
type textEdit struct {
	Range   textRange `json:"range"`
	NewText string    `json:"newText"`
}

// formatting is `canon fmt` of an open .canon buffer: one edit giving the whole text its
// canonical layout, none when it has it. A buffer with a syntax error is refused, as fmt leaves
// such a file unchanged; any other document is null.
func (s *server) formatting(_ context.Context, params json.RawMessage) (any, error) {
	// API.md T1, CLI.md §3.6
	var p formattingParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	abs, ok := pathOf(p.TextDocument.URI)
	if !ok || path.Ext(abs) != project.SourceExt {
		return nil, nil
	}
	s.mu.Lock()
	doc := s.docs[abs]
	s.mu.Unlock()
	if doc == nil {
		return nil, nil
	}
	out, err := formatted(abs, doc.text)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(out, doc.text) {
		return []textEdit{}, nil
	}
	return []textEdit{{Range: textRange{End: newLines(doc.text).end()}, NewText: string(out)}}, nil
}

// formatted is text's canonical layout (FORMATTER.md), errNotFormatted for a syntax error.
func formatted(abs string, text []byte) ([]byte, error) {
	set := &source.FileSet{}
	f, err := set.Add(abs, abs, text)
	if err != nil {
		return nil, err
	}
	out, err := format.Source(f, fileKind(abs), diag.NewBag(set, ""))
	if errors.Is(err, format.ErrSyntax) {
		return nil, fmt.Errorf(fmtWrap, errNotFormatted, err)
	}
	return out, err
}

// fileKind is how a .canon file parses: a project file by its name, else a source (API.md T1).
func fileKind(abs string) syntax.FileKind {
	if path.Base(abs) == project.FileName {
		return syntax.FileProject
	}
	return syntax.FileSource
}
