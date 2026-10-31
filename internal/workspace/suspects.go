package workspace

import (
	"errors"
	"maps"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// Suspect reports names, absolute, changed: the next refresh reads each again with its
// directory's listing, whatever their stamps (API.md S1; DECISIONS 421). It takes only p.mu, a
// leaf lock: callable under any lock but p.mu.
func (p *Project) Suspect(names ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.suspects == nil {
		p.suspects = make(map[string]bool, len(names))
	}
	for _, n := range names {
		p.suspects[n] = true
	}
}

// keepSuspects merges taken, the suspects of a refresh that did not finish, back for the next.
func (p *Project) keepSuspects(taken map[string]bool) {
	if len(taken) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.suspects == nil {
		p.suspects = make(map[string]bool, len(taken))
	}
	maps.Copy(p.suspects, taken)
}

// forcedBy is every name a refresh reads again for suspects: each, and its directory; nil for none.
func forcedBy(suspects map[string]bool) map[string]bool {
	if len(suspects) == 0 {
		return nil
	}
	out := map[string]bool{}
	//canon:unordered a set made from a set
	for n := range suspects {
		out[n], out[project.DirOf(n)] = true, true
	}
	return out
}

// suspectStale suspects each file a *StaleError in err names, at the place s's project gives its
// display (API.md S1, N9; DECISIONS 352, 421).
func (p *Project) suspectStale(s *Snapshot, err error) {
	var names []string
	for _, display := range staleFiles(err) {
		if abs, ok := s.b.Abs(display); ok {
			names = append(names, abs)
		}
	}
	if len(names) > 0 {
		p.Suspect(names...)
	}
}

// staleFiles are the display paths every *StaleError in err names: an edit's, a build's or this
// package's own.
func staleFiles(err error) []string {
	var out []string
	var es *edit.StaleError
	if errors.As(err, &es) {
		out = append(out, es.Files...)
	}
	var bs *build.StaleError
	if errors.As(err, &bs) {
		out = append(out, bs.Files...)
	}
	var ws *StaleError
	if errors.As(err, &ws) {
		out = append(out, ws.Files...)
	}
	return out
}
