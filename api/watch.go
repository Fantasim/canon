package canon

import "context"

// Event reports a new snapshot after a change and its automatic re-check (API.md §12).
type Event struct {
	Revision Revision
	Cause    EventCause
	Files    []string
	Packages []string
	Findings []Finding // all current findings of Packages: replace, do not merge
	Summary  Summary
	Err      error
}

// Watch calls fn after each coalesced change until ctx is done (rules W12-W16).
func (p *Project) Watch(ctx context.Context, fn func(Event)) error {
	return errUnimplemented()
}
