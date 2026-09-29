//go:build !windows

package build

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// syncDir syncs a directory's entries; a file system that cannot is no failure.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	err = d.Sync()
	if errors.Is(err, errors.ErrUnsupported) || errors.Is(err, syscall.EINVAL) {
		err = nil
	}
	return errors.Join(err, d.Close())
}
