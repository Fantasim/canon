package workspace

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// recheck is one kind of entry compared with the disk: the entry read again, or nil when its
// stat shows it cannot have changed and it is not forced (API.md S1).
type recheck func(s *snapFS, n name, e *entry, forced bool) *entry

// rechecks is the recheck of each kind of entry, by kind.
var rechecks = [...]recheck{
	kindFile: func(s *snapFS, n name, e *entry, forced bool) *entry { return s.ifStale(n, e, s.readFile, forced) },
	kindDir:  func(s *snapFS, n name, e *entry, forced bool) *entry { return s.ifStale(n, e, s.readDir, forced) },
	kindStat: func(s *snapFS, n name, _ *entry, _ bool) *entry { return s.stat(n.abs) },
	kindLink: func(s *snapFS, n name, _ *entry, _ bool) *entry { return s.resolve(n.abs) },
}

// ifStale is n read again when forced, a name reported changed (DECISIONS 421), when its stamp
// differs from e's or has no modification time, or when e was read so soon after its
// modification or change time that a change within the same timestamp would not show.
func (s *snapFS) ifStale(n name, e *entry, read func(string) *entry, forced bool) *entry {
	if forced {
		return read(n.abs)
	}
	st := stampOf(s.base.Stat(n.abs))
	racy := st.class == classOK && e.stamp.racy(e.at)
	if st == e.stamp && !st.zero && !racy {
		return nil
	}
	return read(n.abs)
}

// refresh compares every entry but the overlays with the disk, a file or listing forced read
// again (API.md S1). It returns the next file system and the names whose content changed, or nil
// and none; an entry read again with the same content replaces its old one here.
func (s *snapFS) refresh(ctx context.Context, forced map[string]bool) (*snapFS, []name, error) {
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
	return s.compare(ctx, names, olds, forced)
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
	return s.compare(ctx, names, olds, nil)
}

// compare is refresh over names, whose entries are olds, the names of forced read again.
func (s *snapFS) compare(ctx context.Context, names []name, olds []*entry, forced map[string]bool) (*snapFS, []name, error) {
	fresh := make([]*entry, len(names))
	err := inParallel(ctx, len(names), func(i int) {
		fresh[i] = rechecks[names[i].kind](s, names[i], olds[i], forced[names[i].abs])
	})
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
