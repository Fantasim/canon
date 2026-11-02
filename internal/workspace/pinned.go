package workspace

// role is who asks a snapshot for an entry: a reader, a writer pinning it, or a diff between
// two snapshots, which neither reads the program nor writes (DECISIONS 445).
type role uint8

// note records, under s.mu, what a use of n by r says of n: a reader's makes n a read name; a
// writer's first use, or its use of a name only writers pinned, makes n one only a writer pinned,
// which no refresh has found naming nothing yet; a diff's says nothing (API.md S4, DECISIONS 445).
func (s *snapFS) note(n name, r role) {
	switch r {
	case byReader:
		delete(s.onlyPin, n)
	case byWriter:
		_, held := s.ents[n]
		_, flying := s.pending[n]
		if _, only := s.onlyPin[n]; only || !held && !flying {
			if s.onlyPin == nil {
				s.onlyPin = map[name]bool{}
			}
			s.onlyPin[n] = false
		}
	case byDiff:
	}
}

// onlyPinned reports n among the names only a writer pinned on s.
func (s *snapFS) onlyPinned(n name) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, only := s.onlyPin[n]
	return only
}

// absent reports that a refresh's check of an entry old, which gave fresh (nil: its stamp
// unchanged), says its name names nothing (DECISIONS 445).
func absent(old, fresh *entry) bool {
	if fresh == nil {
		return old.stamp.class == classMissing && old.sum.class == classMissing
	}
	return fresh.sum.class == classMissing
}

// goneOf is every names[i] only a writer pinned on s whose check gave fresh[i] and says it names
// nothing: the names the next snapshot does not carry (API.md S4, DECISIONS 445).
func (s *snapFS) goneOf(names []name, olds, fresh []*entry) []name {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []name
	for i, n := range names {
		if _, only := s.onlyPin[n]; only && absent(olds[i], fresh[i]) {
			out = append(out, n)
		}
	}
	return out
}

// markGone notes on s the names a refresh found naming nothing without making a new snapshot,
// so the next snapshot forked from s leaves them (fork); a name read since is a read name.
func (s *snapFS) markGone(gone []name) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range gone {
		if _, only := s.onlyPin[n]; only {
			s.onlyPin[n] = true
		}
	}
}

// carry gives next, forked from s with replaced stored, the names only a writer pinned on s that
// it holds, leaving out each naming nothing that a refresh found so (marked, or in gone), decided
// under s.mu so a name read meanwhile stays; the history records it gone (API.md S4, S9).
func (s *snapFS) carry(next *snapFS, replaced map[name]*entry, gone []name) {
	found := map[name]bool{}
	for _, n := range gone {
		found[n] = true
	}
	//canon:unordered each name is carried or left on its own; the log's order is not read
	for n, marked := range s.onlyPin {
		e, held := next.ents[n]
		_, isReplaced := replaced[n]
		switch {
		case !held:
		case (marked || found[n]) && e.sum.class == classMissing:
			delete(next.ents, n)
			if !isReplaced {
				next.log = append(next.log, n)
			}
		default:
			if next.onlyPin == nil {
				next.onlyPin = map[name]bool{}
			}
			next.onlyPin[n] = false
		}
	}
}

// seen is n's entry in s for a diff from from: s's own, read now when s lacks it, and then kept
// unless only a writer pinned n in from, so a diff never brings back a name a refresh left.
func (s *snapFS) seen(n name, from *snapFS) *entry {
	load := func(a string) *entry { return loaders[n.kind](s, a) }
	if from.onlyPinned(n) && !s.holds(n) {
		return load(n.abs)
	}
	return s.getAs(n, load, byDiff)
}

// holds reports that s has an entry for n.
func (s *snapFS) holds(n name) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ents[n]
	return ok
}
