//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || illumos || windows)

package writelock

import "os"

// lockable: this platform has neither flock nor share modes, so the lock is skipped, advisory
// (DECISIONS 352).
func lockable() bool { return false }

// tryLock is never called where the lock is skipped.
func tryLock(string, Mode) (*os.File, error) { return nil, errBusy }

// readLine is never called where the lock is skipped.
func readLine(string) string { return "" }
