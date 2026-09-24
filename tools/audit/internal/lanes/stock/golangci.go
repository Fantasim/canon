package stock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
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

// runGolangci runs golangci-lint once for every stock rule that is on, on golangci.yml with
// the thresholds rendered in, and maps its issues to findings. A nil, nil result means there
// was nothing to do, not a failure. The whole child, lock wait included, is killed at deadline.
func runGolangci(ctx *lane.Context, deadline time.Duration) ([]finding.Finding, *lane.Skip) {
	wanted := wantedLinters(ctx)
	if len(wanted) == 0 && !ctx.On(ruleFmt) {
		return nil, nil
	}
	targets, ok := golangciTargets(ctx)
	if !ok {
		return nil, nil
	}
	issues, err := lintIssues(ctx, deadline, wanted, targets)
	if err != nil {
		return nil, &lane.Skip{What: skipGolangci, Reason: err.Error()}
	}
	return golangciFindings(ctx, issues), nil
}

// lintIssues runs golangci-lint on targets against the audit's own per-root cache and returns
// its issues, every file root-relative; any error leaves the lane unmeasured.
func lintIssues(ctx *lane.Context, deadline time.Duration, wanted, targets []string) ([]golangciIssue, error) {
	cacheDir, err := golangciCacheDir(ctx.CacheBase, ctx.Repo.Root)
	if err != nil {
		return nil, err
	}
	env, err := golangciExtraEnv(cacheDir)
	if err != nil {
		return nil, err
	}
	bin, err := resolveTool(ctx.Toolchain, toolGolangci)
	if err != nil {
		return nil, err
	}
	cfgPath, err := renderConfig(ctx.Toolchain, ctx.Limits)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(cfgPath) }()
	runCtx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	out, err := runTool(runCtx, bin, ctx.Repo.Root, env, golangciArgs(cfgPath, wanted, targets, deadline)...)
	if err != nil {
		return nil, err
	}
	issues, err := parseGolangciOutput(ctx.Repo.Root, targets, out)
	if errors.Is(err, errOutOfSet) {
		return nil, fmt.Errorf("%w (golangci-lint cache %s)", err, cacheDir)
	}
	return issues, err
}

// parseGolangciOutput decodes golangci-lint's JSON and makes every issue's file root-relative,
// refusing one outside root or outside every target dir before golangciFindings' Skipped-tree
// filter could drop it quietly: it is no finding of this run (a replayed or foreign cache).
func parseGolangciOutput(root string, targets []string, out []byte) ([]golangciIssue, error) {
	var res golangciOutput
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	dirs := targetDirs(targets)
	for i := range res.Issues {
		pos := &res.Issues[i].Pos
		rel, ok := rootRelative(root, pos.Filename)
		if !ok || !dirs[path.Dir(rel)] {
			return nil, fmt.Errorf("%w: %s", errOutOfSet, pos.Filename)
		}
		pos.Filename = rel
	}
	return res.Issues, nil
}

// golangciCacheDir is a cache dir unique to root, under base (never inside the audited tree):
// golangci-lint keys its cache by content hash, so two worktrees sharing one machine-wide
// cache can have a hit replay one worktree's paths into another's report (seen in production).
func golangciCacheDir(base, root string) (string, error) {
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("%w: %q", errCacheBase, base)
	}
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(base, cacheDirName, hex.EncodeToString(sum[:cacheHashBytes])), nil
}

// golangciExtraEnv always points golangci-lint's cache at dir, overriding whatever the caller
// already set: a cache this audit does not control is how a poisoned finding gets in. A dir it
// cannot create fails the lane rather than silently falling back to the shared default cache.
func golangciExtraEnv(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, cacheDirPerm); err != nil {
		return nil, fmt.Errorf("prepare golangci-lint cache dir: %w", err)
	}
	return []string{golangciCacheEnv + "=" + dir}, nil
}

// targetDirs is the root-relative dir of every "./dir" target ("." for the root itself).
func targetDirs(targets []string) map[string]bool {
	dirs := make(map[string]bool, len(targets))
	for _, t := range targets {
		dirs[strings.TrimPrefix(t, dirPrefix)] = true
	}
	return dirs
}

// rootRelative is file as a slash path relative to root, however golangci-lint spelled it
// (relative to root or absolute); ok is false when it resolves outside root's subtree.
func rootRelative(root, file string) (string, bool) {
	abs := filepath.FromSlash(file)
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// renderConfig writes the toolchain's golangci.yml, every `{key}` replaced by its threshold,
// to a temporary file the caller removes.
func renderConfig(toolchain string, limits threshold.Set) (string, error) {
	tmpl, err := os.ReadFile(filepath.Join(toolchain, golangciConfig))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errConfig, err)
	}
	text, err := limits.Expand(string(tmpl))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errConfig, err)
	}
	f, err := os.CreateTemp("", renderedConfigPattern)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errConfig, err)
	}
	_, werr := f.WriteString(text)
	if err := errors.Join(werr, f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("%w: %w", errConfig, err)
	}
	return f.Name(), nil
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
// --rule/--family selection skips the heavy ones without a second config file, and waits for
// a concurrent golangci-lint's lock (argAllowSerialRunners) instead of refusing.
func golangciArgs(cfgPath string, wanted, targets []string, deadline time.Duration) []string {
	args := []string{
		argRun, argConfig, cfgPath, argOutputJSONPath, valStdout, argShowStatsFalse,
		argIssuesExitCodeOK, argConcurrency, argAllowSerialRunners, argTimeout, deadline.String(),
	}
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
	return dirPrefix + dir
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
