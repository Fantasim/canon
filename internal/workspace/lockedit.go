package workspace

import (
	"errors"
	"io/fs"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// ownLocks adds the lock lines the ids the plan adds or retires require (API.md E20), their facts
// from after, the analysis after the edit, added to each lock as it is (log-2026-09-29 M4 U5b-r);
// it reports whether it added a change.
func (o *EditOutcome) ownLocks(after *build.Analysis) (bool, error) {
	ids := make([]build.LockID, len(o.Plan.Locked))
	for i, l := range o.Plan.Locked {
		ids[i] = build.LockID{Name: l.Name, Key: l.Key}
	}
	locks, err := after.LocksWith(ids)
	if err != nil || len(locks) == 0 {
		return false, err
	}
	for _, l := range locks {
		if err := o.lockChange(after, l); err != nil {
			return false, err
		}
	}
	return true, nil
}

// lockChange writes l's content over the lock as it is, refused where it has an overlay (S12).
func (o *EditOutcome) lockChange(after *build.Analysis, l build.Lock) error {
	raw, err := o.Before.fs.ReadFile(l.Abs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	c := edit.Change{Kind: edit.ChangeModified, Path: l.Path, Before: raw, After: l.Content}
	if raw == nil {
		c.Kind = edit.ChangeCreated
	}
	if err := o.Before.noOverlay(after, []edit.Change{c}); err != nil {
		return err
	}
	if i := slices.IndexFunc(o.Changes, func(x edit.Change) bool { return x.Path == c.Path }); i >= 0 {
		o.Changes[i].After = c.After
		return nil
	}
	o.Changes = append(o.Changes, c)
	return nil
}
