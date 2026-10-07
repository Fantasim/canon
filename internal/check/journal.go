package check

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// origin is what produced a finding, so Recheck knows which to report again as they were and
// which to redo: a declaration's check, a file's header, a package's entry keys (E3101), a keyed
// list's entry keys (E3102), or nothing to redo.
type origin struct {
	decl  *object
	file  *syntax.File
	keys  *pkgState
	keyed *object
}

// journal is what a session keeps of a check: each finding with its origin and the files its
// text reads, each fold, and Broken and BrokenViews before finish propagated them (NFR-02).
type journal struct {
	findings []journaled
	folds    []foldCall
	breaks   []Object // each object breakObj broke, in order: what a fold saw is a prefix
	direct   map[Object]bool
	views    map[*syntax.ViewDecl]bool
	hold     bool // Recheck: journal findings without reporting them until it succeeds
	probing  bool // Recheck: journal a fold without asking the Folder, its answer being discarded
}

// journaled is one finding: put reports it again into any bag of its package.
type journaled struct {
	from  origin
	pkg   *pkgState
	put   func(*diag.Bag)
	reads []source.FileID
	body  bool // reported once step 3 began (TYPES.md §1): a declaration checked again reports it again
}

// foldCall is one Fold the checker asked for, when breaks had that many objects.
type foldCall struct {
	owner  Object
	e      syntax.Expr
	breaks int
}

// deliver puts a finding in p's bag; a session journals it first.
func (c *checker) deliver(p *pkgState, from origin, put func(*diag.Bag)) {
	if c.journal == nil {
		put(p.bag)
		return
	}
	c.journal.findings = append(c.journal.findings, journaled{from: from, pkg: p, put: put, reads: readsOf(p.path, put), body: c.bodies})
	if !c.journal.hold {
		put(p.bag)
	}
}

// originOf is the origin of a finding reported in env: the pass's override, else env's declaration.
func (c *checker) originOf(env *env) origin {
	if c.override != nil {
		return *c.override
	}
	return origin{decl: env.owner}
}

// readsOf are the files a finding's rendering reads: its spans, related notes and any location
// or source text in its message.
func readsOf(pkg string, put func(*diag.Bag)) []source.FileID {
	spy := &spyFiles{}
	bag := diag.NewBag(spy, pkg)
	put(bag)
	diag.Locate(spy, bag.Findings())
	slices.Sort(spy.read)
	return slices.Compact(spy.read)
}

// spyFiles knows no file, and notes each one asked about.
type spyFiles struct {
	read []source.FileID
}

func (s *spyFiles) Path(id source.FileID) string {
	s.read = append(s.read, id)
	return ""
}

func (s *spyFiles) Position(id source.FileID, _ source.Pos) (line, col int) {
	s.read = append(s.read, id)
	return 0, 0
}

func (s *spyFiles) Content(id source.FileID) []byte {
	s.read = append(s.read, id)
	return nil
}

// foldJournal is a Folder journaling each call for a session.
type foldJournal struct {
	inner   Folder
	journal *journal
}

func (f *foldJournal) Fold(ctx context.Context, owner Object, e syntax.Expr, info *Info) (value.Value, bool) {
	f.journal.folds = append(f.journal.folds, foldCall{owner: owner, e: e, breaks: len(f.journal.breaks)})
	if f.journal.probing {
		return nil, false
	}
	return f.inner.Fold(ctx, owner, e, info)
}
