package cli

import (
	"fmt"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// renameLine is the JSON object canon rename prints: the keys of canon edit's object (CLI.md §3.16).
type renameLine struct {
	Rename editBody `json:"rename"`
}

// wrapRename is canon rename's line.
func wrapRename(b editBody) any { return renameLine{Rename: b} }

// runRename is `canon rename <name> <new-name>` (CLI.md §3.16): one RenameName op through Project.Edit.
func runRename(inv *invocation) int {
	start := time.Now()
	inv.opt.format = formatJSON // CLI.md §3.16: always JSON lines, failures included
	if len(inv.args) != renameArgs {
		return inv.editFail(fmt.Errorf(fmtArgs, cmdRename, errTwoArgs), start)
	}
	req := canon.Edit{Ops: []canon.Op{canon.RenameName(inv.args[0], inv.args[1])}, DryRun: inv.opt.dryRun}
	return inv.applyEdit(req, start, wrapRename)
}
