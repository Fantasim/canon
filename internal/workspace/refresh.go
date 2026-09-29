package workspace

import (
	"cmp"
	"context"
	"maps"
	"slices"
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
	ents := maps.Clone(s.ents)
	s.mu.Unlock()
	names := slices.SortedFunc(maps.Keys(ents), compareNames)
	changed := map[name]*entry{}
	for _, n := range names {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		e := ents[n]
		if e.over {
			continue
		}
		fresh := rechecks[n.kind](s, n, e)
		switch {
		case fresh == nil:
		case fresh.sum != e.sum:
			changed[n] = fresh
		default:
			s.renew(n, e, fresh)
		}
	}
	if len(changed) == 0 {
		return nil, nil, nil
	}
	return s.fork(s.over, changed), slices.SortedFunc(maps.Keys(changed), compareNames), nil
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
