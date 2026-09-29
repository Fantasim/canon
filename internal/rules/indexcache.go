package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/verify"
)

// IndexCache keeps each file's checks for the next programs of one lineage, one cache per lineage (IMPLEMENTATION-PLAN §7.6).
type IndexCache struct {
	files verify.FileCache[[]*syntax.CheckDecl]
}

// Index is NewIndex, walking only the files c lacks; c forgets the files prog no longer holds.
func (c *IndexCache) Index(prog *check.Program) *Index {
	ix := &Index{files: map[*syntax.CheckDecl]*syntax.File{}, broken: map[types.Type]bool{}, shared: map[*types.VariantType][]*syntax.CheckDecl{}}
	files := c.cache()
	files.KeepOnly(prog)
	if prog == nil {
		return ix
	}
	ix.info, ix.declared = prog.Info, verify.DeclaredTypes(prog)
	for _, pkg := range prog.Packages {
		ix.indexTypes(pkg)
		for _, f := range pkg.Files {
			for _, d := range files.Of(f, checksIn) {
				ix.files[d] = f
			}
		}
	}
	return ix
}

// cache is c's file cache, nil for a nil c.
func (c *IndexCache) cache() *verify.FileCache[[]*syntax.CheckDecl] {
	if c == nil {
		return nil
	}
	return &c.files
}

// checksIn walks f for its check declarations.
func checksIn(f *syntax.File) []*syntax.CheckDecl {
	var out []*syntax.CheckDecl
	syntax.Inspect(f, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CheckDecl); ok {
			out = append(out, c)
		}
		return true
	})
	return out
}
