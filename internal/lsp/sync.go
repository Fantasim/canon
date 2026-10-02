package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// document is an open editor buffer: its URI as the client wrote it, its text, and the project
// root it belongs to, "" outside any project.
type document struct {
	uri  string
	text []byte
	root string
}

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type openParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type changeParams struct {
	TextDocument   textDocumentItem `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

// didOpen makes the buffer an overlay of the project found above it; a file in no project is
// checked for syntax alone.
func (s *server) didOpen(params json.RawMessage) error {
	var p openParams
	if err := decode(params, &p); err != nil {
		return err
	}
	abs, ok := pathOf(p.TextDocument.URI)
	if !ok {
		return nil
	}
	root, findErr := projectAbove(abs)
	s.mu.Lock()
	s.docs[abs] = &document{uri: p.TextDocument.URI, text: []byte(p.TextDocument.Text), root: root}
	if root != "" && s.projects[root] == nil {
		s.projects[root] = newProject(root)
	}
	s.mu.Unlock()
	return errors.Join(findErr, s.synced(abs, root))
}

// didChange replaces the buffer by its last change's text: IMPLEMENTATION-PLAN §8.4 Sync.
func (s *server) didChange(params json.RawMessage) error {
	var p changeParams
	if err := decode(params, &p); err != nil {
		return err
	}
	abs, ok := pathOf(p.TextDocument.URI)
	if !ok || len(p.ContentChanges) == 0 {
		return nil
	}
	s.mu.Lock()
	doc := s.docs[abs]
	if doc != nil {
		doc.text = []byte(p.ContentChanges[len(p.ContentChanges)-1].Text)
	}
	s.mu.Unlock()
	if doc == nil {
		return nil
	}
	return s.synced(abs, doc.root)
}

// didClose drops the buffer: the file is read from the disk again, its findings still published.
func (s *server) didClose(params json.RawMessage) error {
	var p openParams
	if err := decode(params, &p); err != nil {
		return err
	}
	abs, ok := pathOf(p.TextDocument.URI)
	if !ok {
		return nil
	}
	s.mu.Lock()
	doc := s.docs[abs]
	delete(s.docs, abs)
	s.mu.Unlock()
	if doc == nil {
		return nil
	}
	return s.synced(abs, doc.root)
}

// didChangeWatched recomputes every project, a file changed on the disk; one that could not
// open is tried again.
func (s *server) didChangeWatched(json.RawMessage) error {
	s.mu.Lock()
	roots := slices.Sorted(maps.Keys(s.projects))
	s.mu.Unlock()
	var errs []error
	for _, root := range roots {
		errs = append(errs, s.synced(root, root))
	}
	return errors.Join(errs...)
}

// synced brings the project at root up to its buffers, then marks what abs's change made stale:
// the project, or abs itself outside one, and restarts the debounce.
func (s *server) synced(abs, root string) error {
	key := abs
	var err error
	if root != "" {
		key = root
		s.mu.Lock()
		p, docs := s.projects[root], s.buffers(root)
		s.mu.Unlock()
		err = p.sync(docs)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.touch(key)
	return err
}

// touch marks key dirty, cancels the pass running and restarts the debounce; s.mu is held.
func (s *server) touch(key string) {
	s.dirty[key] = true
	if s.cancel != nil {
		s.cancel()
	}
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// projectAbove is the root of the project whose project.canon is nearest above abs, "" for
// none; an error other than finding none is returned too.
func projectAbove(abs string) (string, error) {
	bag := diag.NewBag(&source.FileSet{}, "")
	root, err := project.Find(project.OS(), project.DirOf(abs), bag)
	switch {
	case errors.Is(err, project.ErrNoProject):
		return "", nil
	case err != nil:
		return "", fmt.Errorf(fmtWrap, errFind, err)
	}
	return root, nil
}
