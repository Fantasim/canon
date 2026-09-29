package verify

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// IndexCache keeps each file's scan for the next programs of one lineage, one cache per lineage (IMPLEMENTATION-PLAN §7.6).
type IndexCache struct {
	files FileCache[*fileSyntax]
}

// Index is NewIndex, scanning only the files c lacks; c forgets the files prog no longer holds.
func (c *IndexCache) Index(prog *check.Program) *Index {
	c.cache().KeepOnly(prog)
	return &Index{src: indexSources(prog, c), declared: DeclaredTypes(prog)}
}

func (c *IndexCache) scanned(f *syntax.File) *fileSyntax {
	return c.cache().Of(f, scanFile)
}

func (c *IndexCache) cache() *FileCache[*fileSyntax] {
	if c == nil {
		return nil
	}
	return &c.files
}
