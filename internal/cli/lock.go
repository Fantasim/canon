package cli

import (
	"errors"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// runLock is `canon lock check [packages…]` (CLI.md §3.12, LOCK.md §8): the lock findings, then the summary.
func runLock(inv *invocation) int {
	if len(inv.args) == 0 || inv.args[0] != subLockCheck {
		return inv.fail(fmt.Errorf(fmtArgs, cmdLock, errBadLock))
	}
	inv.args = inv.args[1:]
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	selectors := inv.selectors(p.Root())
	res, err := p.LockCheck(inv.ctx, selectors...)
	if errors.Is(err, canon.ErrUnknownPackage) {
		err = inv.asTyped(p, selectors, err)
	}
	if err != nil {
		return inv.fail(err)
	}
	if err := inv.writeFindings(inv.shown(res.Findings), res.Summary, res.Duration); err != nil {
		return inv.fail(err)
	}
	return inv.checkExit(res.Summary)
}
