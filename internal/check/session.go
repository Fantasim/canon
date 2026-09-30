package check

import (
	"context"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Session is a checked program kept so that an edit re-checks only the files it changed; its
// Program never changes, Recheck returns a new session.
type Session struct {
	lineage *lineage
	c       *checker
	prog    *Program
	journal *journal
}

// lineage is the sessions one CheckSession began: they share one checker, which each Recheck
// moves on, so only the latest can recheck, one call at a time.
type lineage struct {
	mu     sync.Mutex
	latest *Session
}

// CheckSession is Check keeping a Session (IMPLEMENTATION-PLAN §7.6 NFR-02), nil with ctx cancelled.
func CheckSession(ctx context.Context, proj *project.Project, files []*syntax.File, bags Bags, fold Folder) (*Program, *Session) {
	if bags == nil || fold == nil {
		return nil, nil
	}
	j := &journal{}
	c := newChecker(ctx, proj, bags, &foldJournal{inner: fold, journal: j})
	c.journal, c.inputs = j, slices.Clone(files)
	c.run(files)
	prog := c.program()
	if ctx.Err() != nil {
		return prog, nil
	}
	s := &Session{lineage: &lineage{}, c: c, prog: prog, journal: j}
	s.lineage.latest = s
	return prog, s
}

// Recheck is Check over the session's files and project with changed in place of those of their
// paths, bags and fold as Check's; ok is false, nothing reported, when the package doc's terms
// do not hold. The caller then runs Check, as it does whenever project.canon changed.
func (s *Session) Recheck(ctx context.Context, changed []*syntax.File, bags Bags, fold Folder) (*Program, *Session, bool) {
	if s == nil || bags == nil || fold == nil {
		return nil, nil, false
	}
	s.lineage.mu.Lock()
	defer s.lineage.mu.Unlock()
	if s.lineage.latest != s || ctx.Err() != nil {
		return nil, nil, false
	}
	pl, ok := s.c.plan(changed, bags)
	if !ok {
		return nil, nil, false
	}
	next := s.apply(ctx, pl, bags, fold)
	s.lineage.latest = next
	if next == nil {
		return nil, nil, false
	}
	return next.prog, next, true
}

// apply moves the lineage's checker onto the new files, then reports every finding of the new
// program into bags; nil when ctx was cancelled on the way, or a swapped file folded (the replay's
// order would not be cold's), which ends the lineage.
func (s *Session) apply(ctx context.Context, pl *recheckPlan, bags Bags, fold Folder) *Session {
	c := s.c
	j := s.journal.kept(pl)
	folds := j.folds
	c.ctx, c.bags, c.journal = ctx, bags, j
	c.fold = &foldJournal{inner: fold, journal: j}
	c.info = c.info.cloned(j.direct, j.views)
	renew := c.swapFiles(pl)
	j.probing = true // a swapped file must not fold: none is asked, so an abort charges and reports nothing
	c.recheckDecls(pl, renew)
	j.probing = false
	if len(j.folds) != len(folds) {
		return nil // a swapped file folded: the kept folds' order is not cold's, so the caller runs Check
	}
	c.redoKeys(pl)
	c.finish()
	c.renewPackages(byMap(renew))
	prog := c.program()
	if ctx.Err() != nil {
		return nil
	}
	j.hold = false
	for _, f := range j.findings {
		f.put(f.pkg.bag)
	}
	c.replayFolds(ctx, fold, folds)
	return &Session{lineage: s.lineage, c: c, prog: prog, journal: j}
}
