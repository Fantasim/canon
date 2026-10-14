package build

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// Staging is one run's staging token (DECISIONS 351): every file the run writes or sets aside is
// staged under .<base>.<token>.canon-tmp beside its target, so two runs never share a name.
type Staging struct {
	token string
}

// NewStaging draws a run's token: 13 base32 characters, drawn once per run.
func NewStaging() (Staging, error) {
	token, err := newToken()
	return Staging{token: token}, err
}

// WriteAtomic replaces the existing file abs with data through a staged file beside it, the
// file's permissions kept; after a failure the file is as it was and no staged file is left.
func (s Staging) WriteAtomic(fsys WriteFS, abs string, data []byte) error {
	perm, err := permOf(fsys, abs)
	if err != nil {
		return err
	}
	return ReplaceStaged(fsys, abs, stagedAs(abs, s.token), data, perm)
}

// ReplaceStaged writes data over abs through WriteStaged's tmp, then renames tmp over abs; after a failure abs is as it was and tmp is gone (API.md N11).
func ReplaceStaged(fsys WriteFS, abs, tmp string, data []byte, perm *fs.FileMode) error {
	if err := WriteStaged(fsys, tmp, data, perm); err != nil {
		_ = fsys.Remove(tmp) // what a failed stage left, if anything
		return err
	}
	if err := fsys.Rename(tmp, abs); err != nil {
		_ = fsys.Remove(tmp) // the rename failed: abs is untouched
		return err
	}
	return nil
}

// WriteStaged writes data to tmp, first removing any entry there, a link itself, never written through (ADR-0010), then sets perm unless nil.
func WriteStaged(fsys WriteFS, tmp string, data []byte, perm *fs.FileMode) error {
	if err := fsys.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := fsys.WriteFile(tmp, data); err != nil {
		return err
	}
	if perm == nil {
		return nil
	}
	return SetPerm(fsys, tmp, *perm)
}

// newToken is a run's random token: tokenBytes random bytes in base32, without padding.
func newToken() (string, error) {
	var b [tokenBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]), nil
}

// stagedAs is the name abs is staged or set aside under by the run of token, in abs's directory.
func stagedAs(abs, token string) string {
	return project.Join(project.DirOf(abs), stageBase(path.Base(abs), token))
}

// stageBase is the file base's staging name under token, the OS WriteFile's temporary files' too (DECISIONS 351).
func stageBase(base, token string) string { return tempPrefix + base + tempSep + token + tempSuffix }

// fixedStage is 0.1.5's staging name of the file base, .<base>.canon-tmp, which a build still sweeps.
func fixedStage(base string) string { return tempPrefix + base + tempSuffix }

// StagedOf is stagedOf for an edit's recovery, which knows a write's leftovers by it (DECISIONS 351).
func StagedOf(name, base string) bool { return stagedOf(name, base) }

// stagedOf reports name, a directory entry, a staging name of the file base: the name of any
// run's token, or 0.1.5's fixed one, which a build still sweeps (DECISIONS 351).
func stagedOf(name, base string) bool {
	if name == fixedStage(base) {
		return true
	}
	rest, prefixed := strings.CutPrefix(name, tempPrefix+base+tempSep)
	token, suffixed := strings.CutSuffix(rest, tempSuffix)
	return prefixed && suffixed && len(token) == tokenLen && strings.Trim(token, tokenAlphabet) == ""
}

// deadStage reports name a stage of the file base another run left: its staging name with a
// token not this run's, or the temporary file the OS WriteFile wrote that name through.
func (r *run) deadStage(name, base string) bool {
	if stagedOf(name, base) {
		return name != stageBase(base, r.writes().token) && name != fixedStage(base) // 0.1.5's name is the set-aside sweeps' only
	}
	inner, ok := strings.CutPrefix(name, tempPrefix)
	if !ok || len(inner) < len(tempSep)+tokenLen+len(tempSuffix) {
		return false
	}
	inner = inner[:len(inner)-len(tempSep)-tokenLen-len(tempSuffix)]
	return stagedOf(name, inner) && stagedOf(inner, base) && inner != fixedStage(base)
}

// stageName reports a staging or set-aside name, never a file an emit writes.
func stageName(base string) bool {
	return strings.HasPrefix(base, tempPrefix) && strings.HasSuffix(base, tempSuffix)
}

// hidden reports a hidden name, a staged file's among others: never a stray (DECISIONS 356).
func hidden(base string) bool { return strings.HasPrefix(base, tempPrefix) }

// stagedBeside are the staging names of the file abs that its directory lists, of any run, in
// byte order; none when the directory cannot exist.
func (r *run) stagedBeside(abs string) ([]string, error) {
	names, err := r.writes().listing(r.p.fs, project.DirOf(abs))
	if err != nil {
		return nil, err
	}
	base := path.Base(abs)
	var out []string
	for _, name := range names {
		if stagedOf(name, base) {
			out = append(out, project.Join(project.DirOf(abs), name))
		}
	}
	return out, nil
}

// runWrites is what a run knows of its writing: its staging token, the directory listings it
// read, the files it generated per emit and the outputs stage E found colliding (DECISIONS
// 351, 355).
type runWrites struct {
	token     string
	err       error // drawing the token failed
	listings  map[string][]string
	generated map[*ir.Emit]generation
	collided  map[string]bool
	dirCased  map[*ir.Emit]bool // the emits on either side of an E8152 dirCase: never written (DECISIONS 377)
	named     []string          // what the loads named when the run began, which a build re-reads (DECISIONS 352)
}

// generation is what an emit's generator gave.
type generation struct {
	files []ir.File
	err   error
}

// writes is the run's runWrites, made on first use.
func (r *run) writes() *runWrites {
	if r.w == nil {
		token, err := newToken()
		r.w = &runWrites{token: token, err: err, listings: map[string][]string{}, generated: map[*ir.Emit]generation{}, collided: map[string]bool{}, dirCased: map[*ir.Emit]bool{}}
	}
	return r.w
}

// listing is the sorted names of dir, read once per run; none when it cannot exist.
func (w *runWrites) listing(fsys project.FS, dir string) ([]string, error) {
	if names, ok := w.listings[dir]; ok {
		return names, nil
	}
	entries, err := fsys.ReadDir(dir)
	switch {
	case cannotExist(err):
	case err != nil:
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	slices.Sort(names)
	w.listings[dir] = names
	return names, nil
}
