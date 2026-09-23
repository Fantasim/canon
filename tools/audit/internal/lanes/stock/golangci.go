package stock

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

type golangciPos struct {
	Filename string
	Line     int
}

type lineRange struct{ From, To int }

type golangciIssue struct {
	FromLinter string
	Text       string
	Pos        golangciPos
	LineRange  *lineRange
}

type golangciOutput struct {
	Issues []golangciIssue
}

// runGolangci runs golangci-lint once for every stock rule that is on, and maps its issues
// to findings. A nil, nil result means there was nothing to do, not a failure.
func runGolangci(ctx *lane.Context, cfgPath string) ([]finding.Finding, *lane.Skip) {
	wanted := wantedLinters(ctx)
	wantFmt := ctx.On(ruleFmt)
	if len(wanted) == 0 && !wantFmt {
		return nil, nil
	}
	targets, ok := golangciTargets(ctx)
	if !ok {
		return nil, nil
	}
	bin, err := resolveTool(ctx.Toolchain, toolGolangci)
	if err != nil {
		return nil, &lane.Skip{What: skipGolangci, Reason: err.Error()}
	}
	out, err := runTool(bin, ctx.Repo.Root, golangciArgs(cfgPath, wanted, targets)...)
	if err != nil {
		return nil, &lane.Skip{What: skipGolangci, Reason: err.Error()}
	}
	var res golangciOutput
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, &lane.Skip{What: skipGolangci, Reason: fmt.Errorf("parse json: %w", err).Error()}
	}
	return golangciFindings(ctx, res.Issues), nil
}

// wantedLinters is the union of linters behind every stock rule (but fmt) that is on.
func wantedLinters(ctx *lane.Context) []string {
	set := map[string]bool{}
	for rule, linters := range ruleLinters {
		if !ctx.On(rule) {
			continue
		}
		for _, l := range linters {
			set[l] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// golangciArgs builds the run command: only --disable the linters not wanted, so a narrow
// --rule/--family selection skips the heavy ones without a second config file.
func golangciArgs(cfgPath string, wanted, targets []string) []string {
	args := []string{argRun, argConfig, cfgPath, argOutputJSONPath, valStdout, argShowStatsFalse, argIssuesExitCodeOK, argConcurrency}
	if len(wanted) < len(allLinters) {
		args = append(args, argDisable, strings.Join(setDiff(allLinters, wanted), ","))
	}
	return append(args, targets...)
}

func setDiff(all, keep []string) []string {
	keeping := map[string]bool{}
	for _, k := range keep {
		keeping[k] = true
	}
	var out []string
	for _, a := range all {
		if !keeping[a] {
			out = append(out, a)
		}
	}
	return out
}

// golangciTargets is the package dirs of every audited .go file outside --changed mode, else
// those of the changed ones: never ./..., which also matches skipped trees (generated goldens
// that need not compile here). false means there is nothing to lint.
func golangciTargets(ctx *lane.Context) ([]string, bool) {
	if !ctx.Repo.ChangedMode() {
		dirs := goDirs(ctx.Repo.FilesWithExt(repo.GoExt))
		return dirs, len(dirs) > 0
	}
	dirs := changedGoDirs(ctx.Repo)
	return dirs, len(dirs) > 0
}

func changedGoDirs(r *repo.Repo) []string { return goDirs(r.Changed()) }

// goDirs is the sorted ./dir pattern of every .go file in files outside a skipped tree.
func goDirs(files []string) []string {
	set := map[string]bool{}
	for _, f := range files {
		if !strings.HasSuffix(f, repo.GoExt) || repo.Skipped(f) {
			continue
		}
		set[dirPattern(path.Dir(f))] = true
	}
	return slices.Sorted(maps.Keys(set))
}

func dirPattern(dir string) string {
	if dir == "." {
		return "."
	}
	return "./" + dir
}

// golangciFindings maps every issue whose rule is on; formatter issues collapse to one
// finding per file, contextcheck chains to one per final callee counting its call sites.
func golangciFindings(ctx *lane.Context, issues []golangciIssue) []finding.Finding {
	var out []finding.Finding
	seen := map[string]int{}
	for _, iss := range issues {
		rule, ok := mapIssueRule(iss.FromLinter, iss.Text)
		if !ok || rule == "" || !ctx.On(rule) {
			continue
		}
		file := filepath.ToSlash(iss.Pos.Filename)
		if repo.Skipped(file) || ownedByGorules(ctx.Go, file, iss) || dropIssue(ctx.Go, file, rule, iss) {
			continue
		}
		fd, key := golangciFinding(iss, rule, file), collapseKey(rule, file, iss)
		if i, dup := seen[key]; dup && key != "" {
			out[i].Value += fd.Value
			continue
		}
		seen[key] = len(out)
		out = append(out, fd)
	}
	return out
}

// collapseKey groups the issues that make one finding: a file's format issues, the calls
// into one context-dropping function. "" keeps the issue on its own.
func collapseKey(rule, file string, iss golangciIssue) string {
	if rule == ruleFmt {
		return rule + detailSep + file
	}
	if chain := ctxChain(iss.Text); iss.FromLinter == linterContextcheck && len(chain) > 0 {
		return rule + detailSep + chain[len(chain)-1]
	}
	return ""
}

func golangciFinding(iss golangciIssue, rule, file string) finding.Finding {
	text := strings.TrimSpace(iss.Text)
	if chain := ctxChain(text); iss.FromLinter == linterContextcheck && len(chain) > 0 {
		text = fmt.Sprintf(msgCtxPass, chain[len(chain)-1])
	}
	msg, fix := describeIssue(iss.FromLinter, text)
	return finding.Finding{
		Rule:    rule,
		File:    file,
		Line:    iss.Pos.Line,
		Detail:  iss.FromLinter + detailSep + finding.Normalize(text),
		Value:   valueFor(iss.FromLinter, iss),
		Message: msg,
		Fix:     fix,
	}
}
