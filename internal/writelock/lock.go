package writelock

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Mode is how a lock is held: Shared admits other shared holders, Exclusive admits no one.
type Mode uint8

// Request asks for the lock of one package in one mode.
type Request struct {
	Package string
	Mode    Mode
}

// ExclusiveOf are the exclusive requests of pkgs, "" (a file of no package, no package's to
// lock) left out; Acquire sorts them and takes each package once (DECISIONS 352).
func ExclusiveOf(pkgs ...string) []Request {
	var reqs []Request
	for _, p := range pkgs {
		if p != "" {
			reqs = append(reqs, Request{Package: p, Mode: Exclusive})
		}
	}
	return reqs
}

// NoRelease is the release of a writer that took no lock: it frees nothing.
func NoRelease() error { return nil }

// Wait is what a writer about to wait is told: the package, and the line of its lock file, a
// holder's "host pid" ("" when it cannot be read).
type Wait struct {
	Package string
	Holder  string
}

// Held is the set of locks one Acquire took, in name order; Release frees them.
type Held struct {
	reqs  []Request
	files []*os.File
}

type waitingKey struct{}

// WithWaiting is ctx carrying fn, which Acquire calls once, the first time a lock is not free.
func WithWaiting(ctx context.Context, fn func(Wait)) context.Context {
	return context.WithValue(ctx, waitingKey{}, fn)
}

// Acquire takes the lock of each request in dir/.canon/writers, dir being an OS path of the
// project directory: in package-name order, each waited for until free or ctx is done, all or
// none (DECISIONS 352). A platform with no lock primitive holds them all at once, advisory.
func Acquire(ctx context.Context, dir string, reqs []Request) (*Held, error) {
	wanted, err := normalized(reqs)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := &Held{reqs: wanted}
	if !lockable() || len(wanted) == 0 {
		return h, nil
	}
	base, err := writersOf(dir)
	if err != nil {
		return nil, err
	}
	w := &waiter{}
	w.fn, _ = ctx.Value(waitingKey{}).(func(Wait))
	for _, req := range wanted {
		f, err := w.take(ctx, dir, filepath.Join(base, lockPrefix+req.Package), req)
		if err != nil {
			return nil, errors.Join(err, h.Release())
		}
		h.files = append(h.files, f)
	}
	return h, nil
}

// Lockable reports whether this platform has a lock primitive: where it does not, Acquire holds
// its requests vacuously and TryAcquire is ErrNoLocks (DECISIONS 375).
func Lockable() bool { return lockable() }

// Valid is nil when pkg names a plain file of .canon/writers: ErrBadPackage otherwise.
func Valid(pkg string) error {
	if pkg == "" || strings.HasPrefix(pkg, hiddenMark) || strings.ContainsAny(pkg, badNameRune) {
		return fmt.Errorf(fmtAt, ErrBadPackage, pkg)
	}
	return nil
}

// TryAcquire takes the locks without waiting, sorted, all or none; ok is false and nothing held
// when one is held (this process's own too); ErrNoLocks where there is no primitive (DECISIONS 375).
func TryAcquire(dir string, reqs []Request) (h *Held, ok bool, err error) {
	wanted, err := normalized(reqs)
	if err != nil {
		return nil, false, err
	}
	if !lockable() {
		return nil, false, ErrNoLocks
	}
	h = &Held{reqs: wanted}
	if len(wanted) == 0 {
		return h, true, nil
	}
	base, err := writersOf(dir)
	if err != nil {
		return nil, false, err
	}
	for _, req := range wanted {
		name := filepath.Join(base, lockPrefix+req.Package)
		f, err := tryLock(name, req.Mode)
		if errors.Is(err, errBusy) {
			return nil, false, h.Release()
		}
		if err != nil {
			return nil, false, errors.Join(relative(dir, err), h.Release())
		}
		if err := stamp(f, dir); err != nil {
			return nil, false, errors.Join(err, h.Release())
		}
		h.files = append(h.files, f)
	}
	return h, true, nil
}

// Holds reports pkg among the held locks, in mode or a stronger one.
func (h *Held) Holds(pkg string, mode Mode) bool {
	return slices.ContainsFunc(h.reqs, func(r Request) bool { return r.Package == pkg && r.Mode >= mode })
}

// Release frees every lock h holds, the last taken first; a second call frees nothing.
func (h *Held) Release() error {
	var errs []error
	for _, f := range slices.Backward(h.files) {
		if err := f.Close(); err != nil {
			errs = append(errs, fmt.Errorf(fmtWrap, err))
		}
	}
	h.files = nil
	return errors.Join(errs...)
}

// normalized is reqs sorted by package, each package once in its strongest mode.
func normalized(reqs []Request) ([]Request, error) {
	out := slices.Clone(reqs)
	for _, r := range out {
		if err := Valid(r.Package); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(out, func(a, b Request) int { return cmp.Or(cmp.Compare(a.Package, b.Package), cmp.Compare(b.Mode, a.Mode)) })
	return slices.CompactFunc(out, func(a, b Request) bool { return a.Package == b.Package }), nil
}

// writersOf is dir's .canon/writers, each level made if missing, never through a link: one that
// is a link or no directory is ErrUnsafe, one that cannot be made an I/O error (DECISIONS 352).
func writersOf(dir string) (string, error) {
	d := dir
	for seg := range strings.SplitSeq(writersPath, pathSep) {
		d = filepath.Join(d, seg)
		if err := plainDir(d); err != nil {
			return "", relative(dir, err)
		}
	}
	return d, nil
}

// plainDir makes d if it is missing, its parent existing, and refuses anything but a directory.
func plainDir(d string) error {
	info, err := os.Lstat(d)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(d, dirMode); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf(fmtWrap, err)
		}
		info, err = os.Lstat(d)
	}
	switch {
	case err != nil:
		return fmt.Errorf(fmtWrap, err)
	case !info.IsDir():
		return &fs.PathError{Op: opOpen, Path: d, Err: ErrUnsafe}
	}
	return nil
}

// relative is err with the path a *fs.PathError carries made relative to dir, so no absolute
// path reaches a message (DECISIONS 201).
func relative(dir string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return &fs.PathError{Op: pe.Op, Path: rel(dir, pe.Path), Err: pe.Err}
	}
	return fmt.Errorf(fmtWrap, err)
}

// rel is name inside dir, '/'-separated.
func rel(dir, name string) string {
	if r, err := filepath.Rel(dir, name); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.Base(name)
}

// waiter is one Acquire's waiting: the function told, once, whom it waits for.
type waiter struct {
	fn   func(Wait)
	told bool
}

// take is the lock of req, the file name in the project at dir, waited for, its line written
// once it is held.
func (w *waiter) take(ctx context.Context, dir, name string, req Request) (*os.File, error) {
	for {
		f, err := tryLock(name, req.Mode)
		switch {
		case err == nil:
			return f, stamp(f, dir)
		case !errors.Is(err, errBusy):
			return nil, relative(dir, err)
		}
		w.tell(req.Package, name)
		if err := pause(ctx); err != nil {
			return nil, err
		}
	}
}

// tell says once whom the waiter waits for.
func (w *waiter) tell(pkg, name string) {
	if w.fn == nil || w.told {
		return
	}
	w.told = true
	w.fn(Wait{Package: pkg, Holder: readLine(name)})
}

// pause waits one polling period, or until ctx is done.
func pause(ctx context.Context) error {
	t := time.NewTimer(pollEvery)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// stamp writes the holder's line into its lock file, which it keeps open; a failure closes it.
// Shared holders write at once: each line ends at its own length, and the first line read is
// always one holder's whole line.
func stamp(f *os.File, dir string) error {
	host, err := os.Hostname()
	if err != nil {
		host = unknownHost
	}
	line := fmt.Sprintf(lineFormat, host, os.Getpid())
	_, err = f.WriteAt([]byte(line), 0)
	if err == nil {
		err = f.Truncate(int64(len(line)))
	}
	if err != nil {
		return errors.Join(relative(dir, err), f.Close())
	}
	return nil
}

// firstLine is data's first line, without its end.
func firstLine(data []byte) string {
	line, _, _ := strings.Cut(string(data), lineEnd)
	return line
}
