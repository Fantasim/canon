package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/fantasim/canonlang/internal/build"
)

// write replaces each changed file atomically, one at a time: after an interrupt a file is as it was or formatted (CLI.md §2.5).
func (r *fmtRun) write() error {
	for _, c := range r.changes {
		if err := r.inv.ctx.Err(); err != nil {
			return err
		}
		if err := build.WriteAtomic(r.fsys, c.abs, c.updated); err != nil {
			return fmt.Errorf(fmtArgs, c.display, writeCause(err))
		}
	}
	return nil
}

// writeCause is the fixed cause of a write that failed, chosen by errors.Is.
func writeCause(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return errDenied
	}
	return errUnwritable
}
