package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	canon "github.com/fantasim/canonlang/api"
)

// versionJSON is `canon version --format json`, keys in order (IMPLEMENTATION-PLAN.md §8.1).
type versionJSON struct {
	Compiler    string   `json:"compiler"`
	Languages   []string `json:"languages"`
	Fingerprint string   `json:"fingerprint"`
	ViewModel   string   `json:"viewModel"`
	Lock        string   `json:"lock"`
	Commit      string   `json:"commit"`
}

// runVersion is `canon version` (CLI.md §3.14): three lines, or one JSON object.
func runVersion(inv *invocation) int {
	if len(inv.args) > 0 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdVersion, errNoArgs))
	}
	v := canon.Version()
	if inv.opt.format == formatJSON {
		line, err := json.Marshal(map[string]versionJSON{cmdVersion: {
			Compiler: v.Compiler, Languages: v.Languages, Fingerprint: v.Fingerprint,
			ViewModel: v.ViewModel, Lock: v.Lock, Commit: v.Commit,
		}})
		if err != nil {
			return inv.fail(err)
		}
		writeLine(inv.env.Stdout, string(line))
		return exitOK
	}
	commit := v.Commit
	if commit == "" {
		commit = unknownCommit
	}
	_, _ = fmt.Fprintf(inv.env.Stdout, versionFormat, v.Compiler, commit, strings.Join(v.Languages, listSep),
		v.Fingerprint, v.ViewModel, v.Lock)
	return exitOK
}
