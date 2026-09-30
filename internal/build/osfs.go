package build

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/project"
)

// OS is the operating system's file system, writable, each write atomic and synced (API.md §2.2, §10.3).
func OS() WriteFS { return osFS{FS: project.OS()} }

type osFS struct {
	project.FS
}

// WriteFile writes data to a new file beside name, syncs it, renames it over name and syncs the
// directory: name holds its old content or the new, never a part. A file that existed keeps its
// permissions.
func (osFS) WriteFile(name string, data []byte) error {
	return wrapIO(writeSynced(filepath.FromSlash(name), data))
}

// Rename renames, then syncs the directory of newname so that the rename survives a crash.
func (osFS) Rename(oldname, newname string) error {
	target := filepath.FromSlash(newname)
	if err := os.Rename(filepath.FromSlash(oldname), target); err != nil {
		return wrapIO(err)
	}
	return wrapIO(syncDir(filepath.Dir(target)))
}

func (osFS) Remove(name string) error { return wrapIO(os.Remove(filepath.FromSlash(name))) }

func (osFS) Chmod(name string, mode fs.FileMode) error {
	return wrapIO(os.Chmod(filepath.FromSlash(name), mode))
}

func (osFS) MkdirAll(name string) error {
	return wrapIO(os.MkdirAll(filepath.FromSlash(name), dirMode))
}

// DirSyncer is a file system that syncs a directory's entries (API.md §10.3).
type DirSyncer interface {
	SyncDir(dir string) error
}

// SyncDir syncs dir's entries, so that the renames and removals in it survive a crash; nil
// where the system cannot sync a directory, as on Windows.
func (osFS) SyncDir(dir string) error { return wrapIO(syncDir(filepath.FromSlash(dir))) }

// EvalSymlinks lets load.dir follow links through the OS file system a build reads (WIRE.md §6.5).
func (f osFS) EvalSymlinks(name string) (string, error) { return project.EvalSymlinks(f.FS, name) }

// LinkReader is a file system that reads a symbolic link's own text, even where the link leads
// to a name that does not exist.
type LinkReader interface {
	Readlink(name string) (string, error)
}

// Readlink is the text of the symbolic link name, '/'-separated.
func (osFS) Readlink(name string) (string, error) {
	target, err := os.Readlink(filepath.FromSlash(name))
	if err != nil {
		return "", wrapIO(err)
	}
	return filepath.ToSlash(target), nil
}

func wrapIO(err error) error {
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// writeSynced replaces name, or the file its symbolic links lead to, atomically: a synced
// temporary file in its directory renamed over it, then the directory synced. After a failure
// no temporary file is left. On Windows a rename over a file another process holds open fails.
func writeSynced(name string, data []byte) error {
	name, err := linkTarget(name)
	if err != nil {
		return err
	}
	mode, existed := fs.FileMode(fileMode), false
	if info, err := os.Stat(name); err == nil {
		mode, existed = info.Mode().Perm(), true
	}
	tmp, err := tempName(name)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	if err := fill(f, data, existed, mode); err != nil {
		_ = os.Remove(tmp) // the error returned is the write's
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		_ = os.Remove(tmp) // idem
		return fmt.Errorf(fmtWrap, err)
	}
	return syncDir(filepath.Dir(name))
}

// linkTarget is the file name's symbolic links lead to, name itself when it is none; a
// dangling link leads to the file it names, which the write creates.
func linkTarget(name string) (string, error) {
	for range maxLinks {
		if !isLink(name) {
			return name, nil
		}
		target, err := os.Readlink(name)
		if err != nil {
			return "", fmt.Errorf(fmtWrap, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(name), target)
		}
		name = target
	}
	return "", &fs.PathError{Op: linkOp, Path: name, Err: errLinkLoop}
}

// isLink reports a symbolic link at name.
func isLink(name string) bool {
	info, err := os.Lstat(name)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}

// tempName is a new hidden name beside name holding its whole base, so that a recovery finds
// it, and a short random part: name's base, not the temporary part, bounds its length.
func tempName(name string) (string, error) {
	var b [tempRandBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	random := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return filepath.Join(filepath.Dir(name), tempPrefix+filepath.Base(name)+tempSep+random+tempSuffix), nil
}

// fill writes data to f, gives it the mode of the file it replaces, syncs and closes it.
func fill(f *os.File, data []byte, existed bool, mode fs.FileMode) error {
	_, err := f.Write(data)
	if err == nil && existed {
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}
