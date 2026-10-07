package load

import (
	"context"
	"crypto/sha256"
	"maps"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Loader reads every load form against a project's files, one per build run (WIRE.md §6).
type Loader struct {
	FS     project.FS
	Layout *project.Layout
	Set    *source.FileSet
	Reused func(abs string) // when set, told each file a call takes from the Loader's cache, not FS
	// Add, when set, puts a file read, of SHA-256 sum, into Set in place of Set.Add: a cache keeping unchanged files.
	Add func(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error)
	// Kept, when set, is Add unread: sum is FS's, fixed per snapshot (project.SumFile); it notes a file as Add does.
	Kept func(display, abs string, sum [sha256.Size]byte) (*source.File, bool)
	// Globbed, when set, is told each load.dir pattern and its matches' displays, in match order (WIRE.md §10).
	Globbed func(pattern string, matched []string)
	// Parse, when set, parses a JSON source in place of jsonsrc.Parse: a cache of unchanged files' trees.
	Parse func(src *source.File, bag *diag.Bag) (*jsonsrc.Node, error)
	// Headers, when set, keeps each load.defines header's classification while its file is the same.
	Headers *Headers

	mu      sync.Mutex
	headers map[headerKey]*headerFile // by package, then resolved absolute path
	rec     *Inputs                   // the inputs of the load being recorded (Recorded), nil for none
}

// Load reads e against t; false after a finding, poisoning the value (EVALUATION.md §7).
func (l *Loader) Load(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	outer := l.rec
	l.rec = nil // a load a recorded one forces, through a value it reads, is not part of it
	defer func() { l.rec = outer }()
	return l.load(ctx, req, e, t)
}

// load runs e's form.
func (l *Loader) load(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
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

// parse is src's JSON tree, through Parse when set (WIRE.md §3).
func (l *Loader) parse(src *source.File, bag *diag.Bag) (*jsonsrc.Node, error) {
	if l.Parse != nil {
		return l.Parse(src, bag)
	}
	return jsonsrc.Parse(src, bag)
}

// FinishDefines reports each header's W7101, once per package reading it, once every load is forced.
func (l *Loader) FinishDefines() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range slices.SortedFunc(maps.Keys(l.headers), compareHeaderKeys) {
		reportHeaderSkips(l.headers[key])
	}
}
