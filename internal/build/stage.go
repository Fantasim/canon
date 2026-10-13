package build

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
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
	w := &writer{fsys: fsys, token: s.token}
	c := change{abs: abs, data: data, existed: true}
	if err := w.stage(c); err != nil {
		w.removeStages([]change{c})
		w.removeDirs()
		return err
	}
	if err := fsys.Rename(w.stageOf(abs), abs); err != nil {
		w.removeStages([]change{c})
		return err
	}
	return nil
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
	return project.Join(project.DirOf(abs), tempPrefix+path.Base(abs)+tempSep+token+tempSuffix)
}

// StagedOf is stagedOf for an edit's recovery, which knows a write's leftovers by it (DECISIONS 351).
func StagedOf(name, base string) bool { return stagedOf(name, base) }

// stagedOf reports name, a directory entry, a staging name of the file base: the name of any
// run's token, or 0.1.5's fixed one, which a build still sweeps (DECISIONS 351).
func stagedOf(name, base string) bool {
	if name == tempPrefix+base+tempSuffix {
		return true
	}
	rest, prefixed := strings.CutPrefix(name, tempPrefix+base+tempSep)
	token, suffixed := strings.CutSuffix(rest, tempSuffix)
	return prefixed && suffixed && len(token) == tokenLen && strings.Trim(token, tokenAlphabet) == ""
}

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
	named     []string // what the loads named when the run began, which a build re-reads (DECISIONS 352)
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
		r.w = &runWrites{token: token, err: err, listings: map[string][]string{}, generated: map[*ir.Emit]generation{}, collided: map[string]bool{}}
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
