//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly || illumos

package writelock

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// lockable: flock is this platform's primitive (DECISIONS 352).
func lockable() bool { return true }

// tryLock opens the lock file name, creating it, and takes it in mode without waiting: errBusy
// when another holder excludes it. A link or a special file at name is ErrUnsafe.
func tryLock(name string, mode Mode) (*os.File, error) {
	f, err := openPlain(name, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_SH
	if mode == Exclusive {
		how = syscall.LOCK_EX
	}
	if err := flockNow(f, how); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}

// openPlain opens name with flag, never through a symbolic link, and only a regular file.
func openPlain(name string, flag int) (*os.File, error) {
	if info, err := os.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return nil, &os.PathError{Op: opOpen, Path: name, Err: ErrUnsafe}
	}
	f, err := os.OpenFile(name, flag|syscall.O_NOFOLLOW, fileMode)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = &os.PathError{Op: opOpen, Path: name, Err: ErrUnsafe}
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}

// flockNow is flock(how) without blocking: errBusy when another open file holds the lock.
func flockNow(f *os.File, how int) error {
	raw, err := f.SyscallConn()
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	var ferr error
	err = raw.Control(func(fd uintptr) {
		for {
			if ferr = syscall.Flock(int(fd), how|syscall.LOCK_NB); !errors.Is(ferr, syscall.EINTR) {
				return
			}
		}
	})
	switch {
	case err != nil:
		return fmt.Errorf(fmtWrap, err)
	case errors.Is(ferr, syscall.EWOULDBLOCK):
		return errBusy
	case ferr != nil:
		return &os.PathError{Op: opFlock, Path: f.Name(), Err: ferr}
	}
	return nil
}

// readLine is the first line of the lock file name, "" when it cannot be read.
func readLine(name string) string {
	f, err := openPlain(name, os.O_RDONLY)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()                  // a file only read
	data, _ := io.ReadAll(io.LimitReader(f, maxLine)) // what was read before an error is still a holder's
	return firstLine(data)
}
