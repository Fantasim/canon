package repo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

type Repo struct {
	Root   string
	Name   string
	Module string

	files   []string
	base    string
	changed map[string]bool
	states  map[string]rules.Mode
}

// Open resolves root (any directory inside the repo) and loads the repo's file list and state.
func Open(root string) (*Repo, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", root, err)
	}
	if top, gerr := gitOut(abs, "rev-parse", "--show-toplevel"); gerr == nil && !hasGoMod(abs) {
		abs = strings.TrimSpace(top)
	}
	r := &Repo{Root: abs, Name: filepath.Base(abs), Module: readModule(abs)}
	if err := r.loadFiles(); err != nil {
		return nil, err
	}
	states, err := loadStates(filepath.Join(abs, StateFile))
	if err != nil {
		return nil, err
	}
	r.states = states
	return r, nil
}

// Files lists repo-relative, slash-separated paths: tracked plus untracked-not-ignored,
// minus vendored trees and nested modules.
func (r *Repo) Files() []string { return r.files }

// FilesWithExt filters Files by extension (".go", ".md", ...).
func (r *Repo) FilesWithExt(exts ...string) []string {
	var out []string
	for _, f := range r.files {
		for _, e := range exts {
			if strings.HasSuffix(f, e) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

func (r *Repo) Abs(rel string) string { return filepath.Join(r.Root, filepath.FromSlash(rel)) }

// SetChanged narrows the ratchet to files changed against base (plus untracked files).
func (r *Repo) SetChanged(base string) error {
	out, err := gitOut(r.Root, "diff", GitNameOnly, base, "--")
	if err != nil {
		return fmt.Errorf("git diff %s: %w", base, err)
	}
	un, err := gitOut(r.Root, gitLsFiles, "-o", gitExcludeStandard)
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	r.base = base
	r.changed = map[string]bool{}
	for l := range strings.SplitSeq(out+"\n"+un, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			r.changed[l] = true
		}
	}
	return nil
}

func (r *Repo) ChangedMode() bool { return r.changed != nil }

// Base is the revision the ratchet guards against: --base in --changed mode, else HEAD.
func (r *Repo) Base() string {
	if r.base == "" {
		return DefaultBase
	}
	return r.base
}

// InScope says whether a finding on path counts for the ratchet: always outside --changed
// mode; otherwise path itself changed, or path is a directory holding a changed file.
func (r *Repo) InScope(path string) bool {
	if r.changed == nil || r.changed[path] || path == "" || path == "." {
		return true
	}
	prefix := strings.TrimSuffix(path, "/") + "/"
	for c := range r.changed {
		if strings.HasPrefix(c, prefix) && !strings.Contains(c[len(prefix):], "/") {
			return true
		}
	}
	return false
}

// Changed lists the changed files, sorted; nil outside --changed mode.
func (r *Repo) Changed() []string {
	if r.changed == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(r.changed))
}

// Mode is the rule's effective mode here: the repo's state.tsv row, else the default.
func (r *Repo) Mode(rl rules.Rule) rules.Mode {
	if m, ok := r.states[rl.ID]; ok {
		return m
	}
	return rl.Mode
}

func (r *Repo) States() map[string]rules.Mode { return r.states }

// Git runs git in the repo and returns stdout.
func (r *Repo) Git(args ...string) (string, error) { return gitOut(r.Root, args...) }

func (r *Repo) loadFiles() error {
	out, err := gitOut(r.Root, gitLsFiles, "-co", gitExcludeStandard)
	if err != nil {
		return r.walkFiles()
	}
	nested := map[string]bool{}
	for l := range strings.SplitSeq(out, "\n") {
		if dir, ok := strings.CutSuffix(l, goModSuffix); ok {
			nested[dir] = true
		}
	}
	for l := range strings.SplitSeq(out, "\n") {
		if l == "" || Skipped(l) || inNested(l, nested) {
			continue
		}
		if _, serr := os.Lstat(r.Abs(l)); serr != nil {
			continue
		}
		r.files = append(r.files, l)
	}
	sort.Strings(r.files)
	return nil
}

func (r *Repo) walkFiles() error {
	err := filepath.WalkDir(r.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// sovaudit:ignore nil-err -- WalkDir callback deliberately skips unreadable entries
			return nil
		}
		rel, _ := filepath.Rel(r.Root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || Skipped(rel+"/") || hasGoMod(p)) {
				return filepath.SkipDir
			}
			return nil
		}
		r.files = append(r.files, rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", r.Root, err)
	}
	return nil
}

// Skipped reports whether rel (repo-relative, slash-separated) sits under a vendored,
// generated or agent-local tree at any depth, or under an example's generated goldens: the
// one predicate the file walk, a typed load and an external linter's findings all defer to.
func Skipped(rel string) bool {
	for _, s := range skipPrefixes {
		if strings.HasPrefix(rel, s) || strings.Contains(rel, "/"+s) {
			return true
		}
	}
	for _, s := range rootSkipPrefixes {
		if strings.HasPrefix(rel, s) {
			return true
		}
	}
	return strings.HasPrefix(rel, examplesPrefix) &&
		(strings.HasPrefix(rel, fixturesPrefix) || strings.Contains(rel, expectedSegment))
}

func inNested(rel string, nested map[string]bool) bool {
	for d := range nested {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, goModFile))
	return err == nil
}

func readModule(dir string) string {
	f, err := os.Open(filepath.Join(dir, goModFile))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); strings.HasPrefix(l, moduleDirective) {
			return strings.TrimSpace(strings.TrimPrefix(l, moduleDirective))
		}
	}
	return ""
}

func gitOut(dir string, args ...string) (string, error) {
	// sovaudit:ignore security -- exec of the fixed "git" binary, never a resolved/tainted path
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", errors.Join(err, fmt.Errorf("%w: %s", errGitCommand, strings.TrimSpace(errb.String())))
	}
	return strings.TrimRight(out.String(), "\n"), nil
}
