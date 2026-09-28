package encode

import (
	"path"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Assets places the program's asset roots (VIEWMODEL.md 12.3 `asset.root`, 12.9 `assets`): each
// by its display path (WIRE.md 2.3), an unrooted one resolved from the file declaring its type.
type Assets struct {
	dir   string // the project directory
	paths map[*types.AssetSpec]project.Path
	abs   map[string]string // display path -> directory on disk
}

// NewAssets resolves the root of every asset type prog's files declare with layout; without a
// layout every root is written as it is.
func NewAssets(prog *check.Program, layout *project.Layout) *Assets {
	a := &Assets{paths: map[*types.AssetSpec]project.Path{}, abs: map[string]string{}}
	if prog == nil || layout == nil {
		return a
	}
	a.dir = layout.Dir
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				a.declare(prog.Info, layout, f, n)
				return true
			})
		}
	}
	return a
}

// declare resolves the asset type n declares in f, if it is one.
func (a *Assets) declare(info *check.Info, layout *project.Layout, f *syntax.File, n syntax.Node) {
	at, ok := n.(*syntax.AssetType)
	if !ok || info.TypeExprs[at] == nil {
		return
	}
	spec := shape.LayersOf(info.TypeExprs[at]).Asset
	if spec == nil {
		return
	}
	if p, resolved := layout.Resolve(spec.Root, path.Dir(f.Src.Path), f.Span(at), diag.NewBag(nil, "")); resolved {
		a.paths[spec], a.abs[p.Display] = p, p.Abs
	}
}

// Root is spec's root as the model writes it: its display path, as written when unresolved.
func (a *Assets) Root(spec *types.AssetSpec) string {
	if a != nil {
		if p, ok := a.paths[spec]; ok {
			return p.Display
		}
	}
	return spec.Root
}

// Dir is the directory of the root display, relative to the project directory with `/`
// (12.9 `dir`); false when it was not resolved.
func (a *Assets) Dir(display string) (string, bool) {
	abs, ok := a.abs[display]
	if !ok {
		return "", false
	}
	rel, err := filepath.Rel(filepath.FromSlash(a.dir), filepath.FromSlash(abs))
	return filepath.ToSlash(rel), err == nil
}
