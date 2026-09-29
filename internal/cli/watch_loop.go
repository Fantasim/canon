package cli

import (
	"errors"
	"maps"
	"slices"

	canon "github.com/fantasim/canonlang/api"
)

// watcher is the loop of a watched command: the events to answer, the state of the run before, how to run again, and how many times in a row a run answered the previous run's own writes.
type watcher struct {
	inv     *invocation
	events  <-chan canon.Event
	prev    *cycleState
	rerun   func() (*cycleState, error)
	chain   int
	blocked bool
}

// watchLoop runs the command again after each event and prints what changed, until ctx is done: Main then exits 130 (CLI.md §3.3, §3.4, §2.5, IMPLEMENTATION-PLAN.md §8.1).
func (inv *invocation) watchLoop(events <-chan canon.Event, prev *cycleState, rerun func() (*cycleState, error)) int {
	w := &watcher{inv: inv, events: events, prev: prev, rerun: rerun}
	for {
		select {
		case <-inv.ctx.Done():
			return exitOK
		case ev := <-events:
			if err := w.handle(queued(events, ev)); err != nil {
				return inv.fail(err)
			}
		}
	}
}

// queued is ev merged with the events already waiting, one cycle for a burst of changes.
func queued(events <-chan canon.Event, ev canon.Event) canon.Event {
	for {
		select {
		case next := <-events:
			ev = merged(ev, next)
		default:
			return ev
		}
	}
}

// merged is b with the files and packages of a and both errors: a later event replaces an earlier one.
func merged(a, b canon.Event) canon.Event {
	b.Files = union(a.Files, b.Files)
	b.Packages = union(a.Packages, b.Packages)
	b.Err = errors.Join(a.Err, b.Err)
	return b
}

func union(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range append(slices.Clone(a), b...) {
		set[s] = true
	}
	return slices.Sorted(maps.Keys(set))
}

// handle answers one event: it runs the command again and prints the cycle. The error is a failure to write.
func (w *watcher) handle(ev canon.Event) error {
	if ev.Err == nil && len(ev.Files) == 0 && len(ev.Packages) == 0 {
		return nil // a snapshot of nothing the project reads
	}
	own := w.own(ev)
	if own && w.chain >= 1 {
		return w.unsettled(ev)
	}
	cur, err := w.rerun()
	if w.inv.interrupted() {
		return nil // Main reports the interrupt, once
	}
	if err != nil {
		cur = w.inv.failedState(err, w.prev)
	} else if ev.Err != nil {
		w.inv.reportError(ev.Err)
	}
	if own {
		w.chain++
	} else {
		w.chain, w.blocked = 0, false
	}
	prev := w.prev
	w.prev = cur
	return w.inv.writeCycle(ev, prev, cur)
}

// own reports an event caused only by the files the run before wrote: a rebuild that answers it may write again, and a loop of them never ends (meta/decisions/log-2026-09-29.md "M4 U6-r").
func (w *watcher) own(ev canon.Event) bool {
	if len(ev.Files) == 0 || len(w.prev.wrote) == 0 {
		return false
	}
	for _, f := range ev.Files {
		if !w.prev.wrote[f] {
			return false
		}
	}
	return true
}

// unsettled prints, once, that the writes of a build keep causing builds, and waits for a change from outside.
func (w *watcher) unsettled(ev canon.Event) error {
	if w.blocked {
		return nil
	}
	w.blocked = true
	return w.inv.writeUnsettled(ev, w.prev)
}

// interrupted reports that the command was interrupted, whose report is Main's alone (CLI.md §2.5).
func (inv *invocation) interrupted() bool { return inv.ctx.Err() != nil }
