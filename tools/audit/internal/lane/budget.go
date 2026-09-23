package lane

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ToolEnv is the environment every child tool runs with: the parent's plus memory and CPU caps.
func ToolEnv() []string { return append(os.Environ(), toolLimits...) }

// Exclusive blocks until no other audit runs on this machine, so concurrent runs queue
// instead of stacking. The lock is released when the process exits.
func Exclusive(log func(string, ...any)) error {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), lockName), os.O_CREATE|os.O_RDWR, lockPerm)
	if err != nil {
		return fmt.Errorf("open lock: %w", err)
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
		return nil
	}
	log("waiting for another audit run to finish")
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	return nil
}
