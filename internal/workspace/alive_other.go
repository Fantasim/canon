//go:build !windows

package workspace

import (
	"errors"
	"os"
	"syscall"
)

// alive reports a process of this host that runs, or runs as another user (ADR-0010).
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
