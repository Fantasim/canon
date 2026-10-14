package workspace

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/writelock"
)

// settler is the build.Settler of a project on the OS's files, safe for concurrent calls (ADR-0021 J10).
type settler struct {
	site   edit.Site // no call changes it: every snapshot and build sharing it may call it at once
	logger *slog.Logger
}

// Meeting is held grown by the journals of this host it meets, their packages exclusive; held
// itself when none records a package it does not hold exclusive (DECISIONS 375: J4).
func (s settler) Meeting(held []writelock.Request) ([]writelock.Request, error) {
	next, _, err := grow(s.site, held)
	if err != nil {
		return nil, err
	}
	return next, nil
}

// Settle rolls back the journals held covers, and fails while one it meets is left (J4, J6).
func (s settler) Settle(held []writelock.Request) ([]string, error) {
	site := s.site
	site.Held = exclusive(held)
	return settle(site, held, s.logger)
}

// grow is reqs with every package of the journals of this host meeting them requested
// exclusive, and whether one was not held so (DECISIONS 375: J4).
func grow(site edit.Site, reqs []writelock.Request) ([]writelock.Request, bool, error) {
	need, err := edit.Meeting(site, packagesOf(reqs))
	if err != nil {
		return nil, false, err
	}
	held := exclusive(reqs)
	if !slices.ContainsFunc(need, func(p string) bool { return !slices.Contains(held, p) }) {
		return reqs, false, nil
	}
	return append(slices.Clone(reqs), writelock.ExclusiveOf(need...)...), true, nil
}

// settle rolls back, under reqs, the journals site.Held covers, O5's Warn on logger; a rollback
// that fails, or a journal meeting reqs still there after it, is an error wrapping edit.ErrJournal
// that names it, fail closed (API.md N11; DECISIONS 375: J4, J6, J11).
func settle(site edit.Site, reqs []writelock.Request, logger *slog.Logger) ([]string, error) {
	changed, err := edit.Settle(site, logger)
	if err != nil && !errors.Is(err, edit.ErrJournal) {
		err = fmt.Errorf(fmtWrap, edit.ErrJournal, err)
	}
	if err != nil {
		return changed, err
	}
	pkgs := packagesOf(reqs)
	left, err := edit.Meeting(site, pkgs)
	if err != nil || len(left) == 0 {
		return changed, err
	}
	names := meetingJournals(site, pkgs)
	return changed, fmt.Errorf(fmtWrapList, edit.ErrJournal, errMeeting, strings.Join(names, listSep))
}

// journalHead is what meetingJournals reads of a journal: its host and the locks it records.
type journalHead struct {
	Host  string   `json:"host"`
	Locks []string `json:"locks"`
}

// meetingJournals are the names of the journals of this host whose locks meet pkgs, in name order.
func meetingJournals(site edit.Site, pkgs []string) []string {
	jdir := project.Join(site.Layout.Dir(), edit.JournalDir)
	entries, err := site.FS.ReadDir(jdir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		data, err := site.FS.ReadFile(project.Join(jdir, e.Name()))
		var head journalHead
		if err == nil && json.Unmarshal(data, &head) == nil && head.Host == site.Self.Host && meets(head.Locks, pkgs) {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// meets says whether a and b share a package.
func meets(a, b []string) bool {
	return slices.ContainsFunc(a, func(p string) bool { return slices.Contains(b, p) })
}

// strongest is reqs sorted by package, each package once in its strongest mode.
func strongest(reqs []writelock.Request) []writelock.Request {
	out := slices.Clone(reqs)
	slices.SortFunc(out, func(a, b writelock.Request) int {
		return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(b.Mode, a.Mode))
	})
	return slices.CompactFunc(out, func(a, b writelock.Request) bool { return a.Package == b.Package })
}

// exclusive are the packages reqs holds exclusive, sorted and unique.
func exclusive(reqs []writelock.Request) []string {
	var out []string
	for _, r := range reqs {
		if r.Mode == writelock.Exclusive {
			out = append(out, r.Package)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// packagesOf are the packages reqs holds in either mode, sorted and unique.
func packagesOf(reqs []writelock.Request) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.Package
	}
	slices.Sort(out)
	return slices.Compact(out)
}
