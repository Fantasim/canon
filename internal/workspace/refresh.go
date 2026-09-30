package workspace

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// recheck is one kind of entry compared with the disk: the entry read again, or nil when its
// stat shows it cannot have changed (API.md S1).
type recheck func(s *snapFS, n name, e *entry) *entry

// rechecks is the recheck of each kind of entry, by kind.
var rechecks = [...]recheck{
	kindFile: func(s *snapFS, n name, e *entry) *entry { return s.ifStale(n, e, s.readFile) },
	kindDir:  func(s *snapFS, n name, e *entry) *entry { return s.ifStale(n, e, s.readDir) },
	kindStat: func(s *snapFS, n name, _ *entry) *entry { return s.stat(n.abs) },
	kindLink: func(s *snapFS, n name, _ *entry) *entry { return s.resolve(n.abs) },
}

// ifStale is n read again when its stat differs from e's, has no modification time, or e was
// read so soon after that time that a change within the same timestamp would not show.
func (s *snapFS) ifStale(n name, e *entry, read func(string) *entry) *entry {
	st := stampOf(s.base.Stat(n.abs))
	racy := st.class == classOK && e.at.Sub(e.stamp.modTime()) < racyWindow
	if st == e.stamp && !st.zero && !racy {
		return nil
	}
	return read(n.abs)
}

// refresh compares every entry but the overlays with the disk (API.md S1). It returns the
// snapshot's next file system and the names whose content changed, or nil and none; an entry
// read again with the same content replaces its old one here, so it is not read again next time.
func (s *snapFS) refresh(ctx context.Context) (*snapFS, []name, error) {
	s.mu.Lock()
	names := make([]name, 0, len(s.ents))
	olds := make([]*entry, 0, len(s.ents))
	//canon:unordered each entry is compared on its own; the changed names are sorted
	for n, e := range s.ents {
		if !e.over {
			names, olds = append(names, n), append(olds, e)
		}
	}
	s.mu.Unlock()
	return s.compare(ctx, names, olds)
}

// refreshNames is refresh over the entries of only (log-2026-09-29 M4 P18).
func (s *snapFS) refreshNames(ctx context.Context, only []name) (*snapFS, []name, error) {
	s.mu.Lock()
	var names []name
	var olds []*entry
	for _, n := range only {
		if e, ok := s.ents[n]; ok && !e.over {
			names, olds = append(names, n), append(olds, e)
		}
	}
	s.mu.Unlock()
	return s.compare(ctx, names, olds)
}

// compare is refresh over names, whose entries are olds.
func (s *snapFS) compare(ctx context.Context, names []name, olds []*entry) (*snapFS, []name, error) {
	fresh := make([]*entry, len(names))
	err := inParallel(ctx, len(names), func(i int) { fresh[i] = rechecks[names[i].kind](s, names[i], olds[i]) })
	if err != nil {
		return nil, nil, err
	}
	changed := map[name]*entry{}
	for i, n := range names {
		switch f := fresh[i]; {
		case f == nil:
		case f.sum != olds[i].sum:
			changed[n] = f
		default:
			s.renew(n, olds[i], f)
		}
	}
	if len(changed) == 0 {
		return nil, nil, nil
	}
	return s.fork(s.over, changed), slices.SortedFunc(maps.Keys(changed), compareNames), nil
}

// inParallel runs fn for each index below n on refreshWorkers goroutines, each over one run of
// indexes: ctx's error once it is done, or a panic in fn as a *safego.PanicError (X2).
func inParallel(ctx context.Context, n int, fn func(int)) error {
	workers := min(refreshWorkers, n)
	errs := make(chan error, workers)
	for w := range workers {
		from, to := n*w/workers, n*(w+1)/workers
		safego.Go(func() error {
			for i := from; i < to && ctx.Err() == nil; i++ {
				fn(i)
			}
			return nil
		}, func(err error) { errs <- err })
	}
	var first error
	for range workers {
		if err := <-errs; first == nil {
			first = err
		}
	}
	if first != nil {
		return first
	}
	return ctx.Err()
}

// renew stores fresh, read again with e's content, in e's place.
func (s *snapFS) renew(n name, e, fresh *entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ents[n] == e {
		s.ents[n] = fresh
	}
}

func compareNames(a, b name) int {
	return cmp.Or(cmp.Compare(a.abs, b.abs), cmp.Compare(a.kind, b.kind))
}
