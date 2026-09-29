//go:build windows

package workspace

import "os"

// alive reports a process of this host that can still be opened (ADR-0010).
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release() // a handle opened only to probe the pid
	return true
}
