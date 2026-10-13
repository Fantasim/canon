//go:build windows

package writelock

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// lockable: CreateFile share modes are this platform's primitive (DECISIONS 352). Every holder
// opens for reading and writing; an exclusive one shares reading only, so it excludes any other
// holder while a reader of its line still opens it, and a shared one shares both.
func lockable() bool { return true }

// tryLock opens the lock file name, creating it, in mode's share mode: errBusy when another
// handle's share mode excludes it. A link or a special file at name is ErrUnsafe.
func tryLock(name string, mode Mode) (*os.File, error) {
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	if mode == Exclusive {
		share = syscall.FILE_SHARE_READ
	}
	f, err := openPlain(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, share, syscall.OPEN_ALWAYS)
	if err != nil && heldElsewhere(name, err) {
		return nil, errBusy
	}
	return f, err
}

// heldElsewhere reports a failed open of the lock file name that a holder's share mode caused: the file
// opens when no data access is asked, which no share mode refuses, and the failure was no denied
// access. Any other failure is an I/O error, never waited on.
func heldElsewhere(name string, err error) bool {
	if errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrUnsafe) {
		return false
	}
	probe, perr := openPlain(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, syscall.OPEN_EXISTING)
	if perr != nil {
		return false
	}
	_ = probe.Close() // a probe only
	return true
}

// openPlain opens name with CreateFile, never through a reparse point, and only a regular file.
func openPlain(name string, access, share, create uint32) (*os.File, error) {
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, &os.PathError{Op: opOpen, Path: name, Err: err}
	}
	h, err := syscall.CreateFile(ptr, access, share, nil, create, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: opOpen, Path: name, Err: err}
	}
	f := os.NewFile(uintptr(h), name)
	var info syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		err = ErrUnsafe
	}
	if err != nil {
		return nil, errors.Join(&os.PathError{Op: opOpen, Path: name, Err: err}, f.Close())
	}
	return f, nil
}

// readLine is the first line of the lock file name, "" when it cannot be read.
func readLine(name string) string {
	f, err := openPlain(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, syscall.OPEN_EXISTING)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()                  // a file only read
	data, _ := io.ReadAll(io.LimitReader(f, maxLine)) // what was read before an error is still a holder's
	return firstLine(data)
}
