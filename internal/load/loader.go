package load

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// Loader reads every load form against a project's files, one per build run (WIRE.md §6).
type Loader struct {
	FS     project.FS
	Layout *project.Layout
	Set    *source.FileSet

	mu      sync.Mutex
	headers map[string]*headerFile // by resolved absolute path
}

// Request is a load forced from one file: package, directory, site, bag, decoding host (DECISIONS 173).
type Request struct {
	Pkg     string
	From    string
	Span    source.Span
	Bag     *diag.Bag
	Host    wire.Host
	Coll    *types.Collection // the let collection the load is the whole value of (wire.Decoder.Coll)
	Outer   wire.Outer        // what the loaded type's arguments name around the load
	Scratch bool              // Bag is thrown away (a test call's, a vector's): the Loader caches nothing for it
}

// decoder is the wire decoder of req's load.
func (req Request) decoder(partial bool) *wire.Decoder {
	return &wire.Decoder{Bag: req.Bag, Pkg: req.Pkg, Host: req.Host, Partial: partial, Coll: req.Coll, Outer: req.Outer}
}

// Load reads e against t; false after a finding, poisoning the value (EVALUATION.md §7).
func (l *Loader) Load(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	form := loadForm
	if e.Method != nil {
		form = e.Method.Name
	}
	switch form {
	case loadForm:
		return l.bare(ctx, req, e, t)
	case methodDir:
		return l.dir(ctx, req, e, t)
	case methodCSV:
		return l.csv(ctx, req, e, t)
	case methodText:
		return l.text(ctx, req, e, t)
	case formDefines:
		return l.defines(ctx, req, e, t)
	default:
		return nil, false, unsupported(causeUnknownForm)
	}
}

// FinishDefines reports each header's W7101 once every load of the build is forced (WIRE.md §6.8).
func (l *Loader) FinishDefines() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, abs := range slices.Sorted(maps.Keys(l.headers)) {
		reportHeaderSkips(l.headers[abs])
	}
}
