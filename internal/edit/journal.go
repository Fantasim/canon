package edit

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// journal is what a commit writes before it changes any file: its writer, every file it touches
// with its previous content (or absence) and the digest of its new one, and the directories it
// creates and removes (API.md N10, N11, O5; log-2026-09-29 M4 U4c-r).
type journal struct {
	Version  int           `json:"version"`
	Revision string        `json:"revision"`
	Host     string        `json:"host"`
	PID      int           `json:"pid"`
	Files    []journalFile `json:"files"`
	Created  []string      `json:"created,omitempty"`
	Removed  []removedDir  `json:"removed,omitempty"`
}

// journalFile is one file before the commit (Old when it Existed, its rw permissions if known)
// and after it (New: the SHA-256 of its content, or newAbsent). Path is relative to the project
// directory, or absolute under a declared root elsewhere.
type journalFile struct {
	Path    string  `json:"path"`
	Existed bool    `json:"existed"`
	Old     []byte  `json:"old,omitempty"`
	Mode    *uint32 `json:"mode,omitempty"`
	New     string  `json:"new"`
}

// removedDir is a directory a commit removes, with its permissions if known.
type removedDir struct {
	Path string  `json:"path"`
	Mode *uint32 `json:"mode,omitempty"`
}

// fault is why a journal, or the commit that would write it, is refused: a reason and the paths.
type fault struct {
	reason, name string
}

func (f *fault) Error() string { return f.reason + detailSep + f.name }

// refusal is err under sentinel when it is a fault; any other error stays as it is.
func refusal(sentinel, err error) error {
	var f *fault
	if errors.As(err, &f) {
		return fmt.Errorf(fmtBadMember, sentinel, err)
	}
	return err
}

// journalName is the journal of the commit whose new revision is rev, <scheme>:<hex>: named by
// its hex part (log-2026-09-29 "Journal file name").
func journalName(dir, rev string) (string, error) {
	i := strings.IndexByte(rev, revisionSep)
	if i < 1 || !isLowerHex(rev[i+1:]) {
		return "", fmt.Errorf(fmtFileErr, rev, ErrRevision)
	}
	return path.Join(dir, journalDir, rev[i+1:]+ir.JSONExt), nil
}

// isJournal says whether a name in the journal directory is a journal, not a file the FS's own
// atomic write left.
func isJournal(name string) bool {
	sum, ok := strings.CutSuffix(name, ir.JSONExt)
	return ok && isLowerHex(sum)
}

func isLowerHex(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) > 0 && hex.EncodeToString(b) == s
}

// digest is the New of a file whose content is data.
func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// encode is the journal's JSON, one line: struct fields in declaration order, files and
// directories already in byte order, so the same commit gives the same bytes.
func (j *journal) encode() ([]byte, error) {
	data, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf(fmtFileErr, j.Revision, err)
	}
	return append(data, lineEnd...), nil
}

// resolve checks a journal, untrusted since Recover runs at every Open, and places what it
// undoes; any fault refuses it whole (log-2026-09-29 M4 U4c-r, U4c-r3).
func (j *journal) resolve(at Layout) (*undo, error) {
	if j.Version != journalVersion {
		return nil, &fault{reasonVersion, j.Revision}
	}
	u := &undo{}
	seen := map[string]bool{}
	for _, f := range j.Files {
		s, reason := confine(at, f.Path)
		switch {
		case reason == "" && seen[s.abs]:
			reason = reasonTwice
		case reason == "":
			reason = f.fault()
		}
		if reason != "" {
			return nil, &fault{reason, f.Path}
		}
		seen[s.abs] = true
		u.files = append(u.files, undoFile{spot: s, journalFile: f})
		u.stages = append(u.stages, s.abs)
	}
	if err := u.resolveDirs(at, j); err != nil {
		return nil, err
	}
	return u, nil
}

// fault is what makes a journal file impossible to have been written by a commit, "" for nothing.
func (f journalFile) fault() string {
	switch {
	case !editable(f.Path):
		return reasonKind
	case f.Mode != nil && *f.Mode&^rwBits != 0:
		return reasonMode
	case f.New != newAbsent && (len(f.New) != hex.EncodedLen(sha256.Size) || !isLowerHex(f.New)):
		return reasonDigest
	case !f.Existed && (len(f.Old) > 0 || f.Mode != nil):
		return reasonAbsent
	}
	return ""
}

// editable says whether name is a file the edit API writes: a source, a JSON file, a lock.
func editable(name string) bool {
	switch path.Ext(name) {
	case project.SourceExt, ir.JSONExt, lockExt:
		return true
	}
	return false
}

// resolveDirs places the directories a journal creates, each above a file that did not exist,
// and those it removes.
func (u *undo) resolveDirs(at Layout, j *journal) error {
	for _, d := range j.Created {
		s, reason := confine(at, d)
		if reason == "" && !u.holdsNew(s.abs) {
			reason = reasonOrphan
		}
		if reason != "" {
			return &fault{reason, d}
		}
		u.created = append(u.created, s)
	}
	for _, d := range j.Removed {
		s, reason := confine(at, d.Path)
		if reason == "" && d.Mode != nil && *d.Mode&^uint32(fs.ModePerm) != 0 {
			reason = reasonMode
		}
		if reason != "" {
			return &fault{reason, d.Path}
		}
		u.removed = append(u.removed, undoDir{spot: s, mode: d.Mode})
	}
	return nil
}

// holdsNew says whether dir is above a file the journal's commit created.
func (u *undo) holdsNew(dir string) bool {
	for _, f := range u.files {
		if _, in := below(dir, f.abs); in && !f.Existed {
			return true
		}
	}
	return false
}

// spot is a place a journal names: the file or directory, and the project directory or declared
// root it lies strictly inside.
type spot struct {
	abs, base string
}

// confine places a journal path strictly inside the project directory or a declared root, with no
// hidden segment below that base; the reason it refuses one, "" for none (log-2026-09-29 M4 U4c-r3).
func confine(at Layout, name string) (spot, string) {
	s, rel, ok := locate(at, name)
	switch {
	case !ok:
		return spot{}, reasonPlace
	case !visible(rel):
		return spot{}, reasonHidden
	}
	return s, ""
}

// locate is the spot a journal path names and its path below the base: relative inside the project
// directory, or absolute inside it or a declared root, strictly.
func locate(at Layout, name string) (spot, string, bool) {
	dir := path.Clean(at.Dir())
	if !isAbsName(name) {
		abs := path.Join(dir, name)
		local := filepath.IsLocal(filepath.FromSlash(name)) && name == path.Clean(name) && abs != dir
		return spot{abs, dir}, name, local
	}
	if name != path.Clean(name) {
		return spot{}, "", false
	}
	if rel, in := below(dir, name); in {
		return spot{name, dir}, rel, true
	}
	root, rest, nested := strings.Cut(at.Display(name), pathSep)
	if !nested || !strings.HasPrefix(root, string(rootMark)) {
		return spot{}, "", false
	}
	back, ok := at.Abs(root + pathSep + rest)
	base, isRoot := at.Abs(root)
	return spot{name, base}, rest, ok && isRoot && back == name
}

// visible says whether no segment of a relative path is hidden: commits never write one.
func visible(rel string) bool {
	for seg := range strings.SplitSeq(rel, pathSep) {
		if seg == "" || seg[0] == hiddenMark {
			return false
		}
	}
	return true
}

func isAbsName(name string) bool {
	return path.IsAbs(name) || filepath.IsAbs(filepath.FromSlash(name))
}

// below is abs relative to dir, if abs lies strictly inside it.
func below(dir, abs string) (string, bool) {
	rel, ok := strings.CutPrefix(abs, strings.TrimSuffix(dir, pathSep)+pathSep)
	return rel, ok && rel != ""
}

// relIn is how a journal names abs: relative inside the project directory, else absolute
// (log-2026-09-29 M4 U4c-r).
func relIn(dir, abs string) string {
	if rel, in := below(dir, abs); in {
		return rel
	}
	return abs
}

// linkResolver is a file system that resolves symbolic links, as load.dir's (WIRE.md §6.5).
type linkResolver interface {
	EvalSymlinks(name string) (string, error)
}

// straight refuses spots with a symbolic link below their base: no component a listing shows as
// a link, dangling or not, and, where the FS resolves links, the deepest existing part resolving
// to itself (log-2026-09-29 M4 U4c-r3).
func straight(fsys build.WriteFS, spots []spot) error {
	listings := map[string][]fs.DirEntry{}
	r, resolves := fsys.(linkResolver)
	for _, s := range spots {
		err := noLinkListed(fsys, listings, s)
		if err == nil && resolves {
			err = straightOne(r, s)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// noLinkListed walks s down from its base, refusing it at the first component its parent's
// listing shows as a symbolic link; listings are kept per directory.
func noLinkListed(fsys build.WriteFS, listings map[string][]fs.DirEntry, s spot) error {
	rel, _ := below(s.base, s.abs)
	dir := s.base
	for seg := range strings.SplitSeq(rel, pathSep) {
		entries, seen := listings[dir]
		if !seen {
			var err error
			if entries, err = fsys.ReadDir(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf(fmtFileErr, dir, err)
			}
			listings[dir] = entries
		}
		i := slices.IndexFunc(entries, func(e fs.DirEntry) bool { return e.Name() == seg })
		if i < 0 {
			return nil
		}
		if entries[i].Type()&fs.ModeSymlink != 0 {
			return &fault{reasonLink, s.abs}
		}
		dir = path.Join(dir, seg)
	}
	return nil
}

func straightOne(r linkResolver, s spot) error {
	real, err := r.EvalSymlinks(s.base)
	if err != nil {
		return fmt.Errorf(fmtFileErr, s.base, err)
	}
	rel, _ := below(s.base, s.abs)
	for p := s.abs; p != s.base && p != path.Dir(p); p, rel = path.Dir(p), path.Dir(rel) {
		got, err := r.EvalSymlinks(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf(fmtFileErr, p, err)
		}
		if got != path.Join(real, rel) {
			return &fault{reasonLink, s.abs}
		}
		return nil
	}
	return nil
}

// replace writes data over abs through a stage beside it; after a failure no stage is left.
func replace(fsys build.WriteFS, abs string, data []byte, mode *uint32) error {
	if err := stage(fsys, abs, data, mode); err != nil {
		_ = fsys.Remove(stageName(abs)) // what a failed stage left, if anything
		return err
	}
	if err := fsys.Rename(stageName(abs), abs); err != nil {
		_ = fsys.Remove(stageName(abs)) // the rename failed: the target is untouched
		return err
	}
	return nil
}

// stage writes data to abs's stage, with the permissions mode where it is known and the FS sets them.
func stage(fsys build.WriteFS, abs string, data []byte, mode *uint32) error {
	tmp := stageName(abs)
	if err := fsys.WriteFile(tmp, data); err != nil {
		return err
	}
	return chmod(fsys, tmp, mode)
}

// chmod gives name the permissions mode, when known and the FS sets them.
func chmod(fsys build.WriteFS, name string, mode *uint32) error {
	m, ok := fsys.(build.ModeFS)
	if !ok || mode == nil {
		return nil
	}
	return m.Chmod(name, fs.FileMode(*mode).Perm())
}

// stageName is the file a commit writes abs's new content to before renaming it over abs.
func stageName(abs string) string {
	return path.Join(path.Dir(abs), stagePrefix+path.Base(abs)+stageSuffix)
}

// deepestFirst orders directories so each comes before its parents: longest first, then bytes.
func deepestFirst(a, b string) int {
	return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b))
}

// rwOnly is a file's permissions as a journal keeps them: read and write bits only, so an exec
// bit is not restored after a crash (log-2026-09-29 M4 U4c-r3).
func rwOnly(mode *uint32) *uint32 {
	if mode == nil {
		return nil
	}
	m := *mode & rwBits
	return &m
}

// permOf is info's permissions in journal form.
func permOf(info fs.FileInfo) *uint32 {
	m := uint32(info.Mode().Perm())
	return &m
}
