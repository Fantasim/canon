package typedef

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// World is the program a typeFunction's `drivers` are read over and its collections' entries:
// every package that may pass a collection to the package's type functions, whatever the build
// selects (VIEWMODEL.md 12.3, J5).
type World struct {
	Program *check.Program
	Colls   *encode.Colls
}

// drivers is the World drivers are read over, resolved on its first use; nil once resolving
// it failed (Err).
func (s *Types) drivers() *World {
	if s.world == nil && s.err == nil {
		s.world, s.err = s.in.Drivers()
	}
	return s.world
}

// Err is what resolving the drivers' program met, or a type function that program lacks: a
// compiler bug, the same sources declaring it (VIEWMODEL.md 12.3).
func (s *Types) Err() error { return s.err }

// inWorld is fn as w declares it: fn itself over the model's own program, else the type function
// of the same package and name; nil, and Err set, when w has none.
func (s *Types) inWorld(w *World, fn *types.TypeFunc) *types.TypeFunc {
	if w.Program == s.in.Program {
		return fn
	}
	if o := s.objects[fn]; o != nil {
		if tf := declaredFunc(w.Program, o.Pkg(), o.Name()); tf != nil {
			return tf
		}
	}
	s.err = fmt.Errorf(fmtNoTypeFunc, ErrNoTypeFunc, fn.Name)
	return nil
}

// declaredFunc is the type function name of package pkg in prog, nil when it has none.
func declaredFunc(prog *check.Program, pkg, name string) *types.TypeFunc {
	p := shape.Package(prog, pkg)
	if p == nil {
		return nil
	}
	for _, d := range p.Decls {
		if d.Kind() != check.ObjTypeName || d.Name() != name || d.Type() == nil {
			continue
		}
		if tf, ok := key(d.Type()).(*types.TypeFunc); ok {
			return tf
		}
	}
	return nil
}
