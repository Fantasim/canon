package cli

import (
	"regexp"
	"strings"
)

// findingHeads matches the first token of a finding, `error[E3501]` or `warning[W1002]`, at the
// start of a line; detail lines are indented and never match.
var findingHeads = regexp.MustCompile(findingHead)

// colored is CLI.md §2.3: never in JSON, --color always or never as given, auto on a terminal without NO_COLOR.
func (inv *invocation) colored() bool {
	if inv.opt.format == formatJSON {
		return false
	}
	switch inv.opt.color {
	case colorAlways:
		return true
	case colorNever:
		return false
	}
	return inv.env.Terminal && !inv.env.NoColor
}

// paint colours the severity word and code of each finding: red for an error, yellow for a warning.
func (inv *invocation) paint(rendered string) string {
	if !inv.colored() {
		return rendered
	}
	return findingHeads.ReplaceAllStringFunc(rendered, func(head string) string {
		color := ansiRed
		if strings.HasPrefix(head, severityWarn) {
			color = ansiYellow
		}
		return color + head + ansiReset
	})
}
