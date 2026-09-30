package workspace

import (
	"cmp"
	"context"
	"io/fs"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
)

// listing is a snapshot's read-set listing (API.md S3) as last computed; a later one, on it or on
// a fork, keeps the scan, the members and each member's entry while what they read is unchanged
// (log-2026-09-29 M4 P18).
type listing struct {
	rev      string
	inputs   []build.Read // the build's inputs (build.Project.Inputs)
	scanned  []scanned    // every directory the scan listed, in order
	scanErr  bool
	set      *inputSet    // the files loads read, as it took them
	recorded []build.Read // set, by absolute name
	members  []member     // each name read, once, in the order of the reads: inputs then recorded
	order    []int        // members, by display
}

// scanned is a directory the scan listed and its listing's key.
type scanned struct {
	abs string
	sum sum
}

// member is one name of the read set, by the display it is listed under, and its entry.
type member struct {
	display, abs string
	e            *entry
}

// list is the revision of s's read set as it holds it now (S3).
func (s *Snapshot) list(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	was := s.fs.listed()
	l := &listing{}
	l.inputs, l.scanned, l.scanErr = s.scan(was)
	l.set, l.recorded = s.fs.recordedFor(was)
	same := was != nil && sameReads(was, l)
	if same {
		l.members, l.order = was.members, was.order
	} else {
		l.members, l.order = membersOf(l.inputs, l.recorded)
	}
	members, changed, err := s.fs.entriesNow(ctx, l.members)
	if err != nil {
		return "", err
	}
	l.members = members
	if same && !changed {
		l.rev = was.rev
	} else {
		l.rev = build.RevisionOf(l.lines())
	}
	s.fs.keepListing(l)
	return l.rev, nil
}

// scan is the build's inputs, the directories the scan listed and whether it failed: was's while
// every directory it listed has the same key here (project.Scan reads only names and kinds).
func (s *Snapshot) scan(was *listing) ([]build.Read, []scanned, bool) {
	if was != nil && s.fs.sameDirs(was.scanned) {
		return was.inputs, was.scanned, was.scanErr
	}
	rec := &scanRecorder{fs: s.fs}
	reads, err := s.p.tmpl.Over(rec).Inputs()
	return reads, rec.dirs, err != nil
}

// sameDirs reports each directory of dirs listed with the same key in s.
func (s *snapFS) sameDirs(dirs []scanned) bool {
	for _, d := range dirs {
		if s.get(name{kind: kindDir, abs: d.abs}, s.readDir).sum != d.sum {
			return false
		}
	}
	return true
}

// sameReads reports l reading the same names under the same displays as was.
func sameReads(was, l *listing) bool {
	return was.set == l.set && slices.Equal(was.inputs, l.inputs) && was.scanErr == l.scanErr
}

// membersOf is each name of inputs then recorded, once, the first display kept, and their order
// by display, ties in the order of the reads.
func membersOf(inputs, recorded []build.Read) ([]member, []int) {
	seen := map[string]bool{}
	var out []member
	for _, reads := range [...][]build.Read{inputs, recorded} {
		for _, r := range reads {
			if !seen[r.Abs] {
				seen[r.Abs] = true
				out = append(out, member{display: r.Display, abs: r.Abs})
			}
		}
	}
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(out[a].display, out[b].display) })
	return out, order
}

// entriesNow is members with the entries s holds now, read if not yet, and whether a key changed;
// members itself when every entry is the one it holds.
func (s *snapFS) entriesNow(ctx context.Context, members []member) ([]member, bool, error) {
	var stale []int
	s.mu.Lock()
	for i, m := range members {
		if e, ok := s.ents[name{kind: kindFile, abs: m.abs}]; !ok || e != m.e {
			stale = append(stale, i)
		}
	}
	s.mu.Unlock()
	if len(stale) == 0 {
		return members, false, nil
	}
	out, changed := slices.Clone(members), false
	for _, i := range stale {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		e := s.get(name{kind: kindFile, abs: out[i].abs}, s.readFile)
		changed = changed || out[i].e == nil || out[i].e.sum != e.sum
		out[i].e = e
	}
	return out, changed, nil
}

// lines is the listing's lines by display, a file that does not exist left out; in the order of
// the reads when two share a display or the scan failed, which RevisionOf then sorts as it does.
func (l *listing) lines() []build.Listed {
	if l.scanErr {
		return l.readOrder()
	}
	out := make([]build.Listed, 0, len(l.members))
	for _, i := range l.order {
		line, ok := l.members[i].line()
		if !ok {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Display == line.Display {
			return l.readOrder()
		}
		out = append(out, line)
	}
	return out
}

// readOrder is the listing's lines in the order of the reads, a failed scan's line last.
func (l *listing) readOrder() []build.Listed {
	out := make([]build.Listed, 0, len(l.members)+1)
	for _, m := range l.members {
		if line, ok := m.line(); ok {
			out = append(out, line)
		}
	}
	if l.scanErr {
		out = append(out, build.Listed{Display: listingDisplay, Unreadable: true})
	}
	return out
}

// line is m's line of the listing, false for a file that does not exist.
func (m member) line() (build.Listed, bool) {
	switch m.e.sum.class {
	case classOK:
		return build.Listed{Display: m.display, Sum: m.e.sum.hash}, true
	case classUnreadable:
		return build.Listed{Display: m.display, Unreadable: true}, true
	case classMissing, classGone:
	}
	return build.Listed{}, false
}

// listed is the listing last computed on s, or on the snapshot it was forked from; nil for none.
func (s *snapFS) listed() *listing {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lst
}

// keepListing keeps l for the next computation on s or on a snapshot forked from it.
func (s *snapFS) keepListing(l *listing) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lst = l
}

// recordedFor is the files loads read, shared from now on so that they change only in a copy,
// and their reads: was's when it took the same set.
func (s *snapFS) recordedFor(was *listing) (*inputSet, []build.Read) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inShared = true
	if was != nil && was.set == s.inputs {
		return was.set, was.recorded
	}
	return s.inputs, s.inputs.reads()
}

// scanRecorder is a snapshot's file system that notes each directory a scan lists, with its key.
type scanRecorder struct {
	fs   *snapFS
	dirs []scanned
}

func (r *scanRecorder) ReadFile(abs string) ([]byte, error) { return r.fs.ReadFile(abs) }

func (r *scanRecorder) Stat(abs string) (fs.FileInfo, error) { return r.fs.Stat(abs) }

func (r *scanRecorder) ReadDir(abs string) ([]fs.DirEntry, error) {
	e := r.fs.get(name{kind: kindDir, abs: abs}, r.fs.readDir)
	r.dirs = append(r.dirs, scanned{abs: abs, sum: e.sum})
	return slices.Clone(e.list), e.err
}
