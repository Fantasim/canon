package project

import (
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

func rootClutter(ctx *lane.Context) []finding.Finding {
	files := ctx.Repo.Files()
	extra := rootAllow(ctx.Repo)
	var out []finding.Finding
	for _, name := range topLevelEntries(files) {
		if !rootAllowed(name, files, extra) {
			out = append(out, finding.Finding{Rule: ruleRootClutter, File: name, Message: rootClutterMessage})
		}
	}
	return out
}

// rootAllow reads .sovaudit/root-allow.txt: one root entry per line, # comments and blanks
// ignored.
func rootAllow(r *repo.Repo) map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(r.Abs(repo.RootAllowFile))
	if err != nil {
		return out
	}
	for l := range strings.SplitSeq(string(data), lineBreak) {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, commentMark) {
			out[strings.TrimSuffix(l, pathSep)] = true
		}
	}
	return out
}

func topLevelEntries(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		name, _, _ := strings.Cut(f, pathSep)
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// rootAllowed checks the fixed allowlists, the repo's own list, a root .go file, and a
// directory holding Go packages at any depth.
func rootAllowed(name string, files []string, extra map[string]bool) bool {
	if slices.Contains(rootAllowedDirs, name) || slices.Contains(rootAllowedFiles, name) || extra[name] {
		return true
	}
	if strings.HasSuffix(name, goExt) {
		return true
	}
	prefix := name + pathSep
	return slices.ContainsFunc(files, func(f string) bool {
		return strings.HasPrefix(f, prefix) && strings.HasSuffix(f, goExt)
	})
}
