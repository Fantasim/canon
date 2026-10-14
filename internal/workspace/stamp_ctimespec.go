//go:build darwin || freebsd || netbsd

package workspace

import "syscall"

// sysStamp is the change time, inode and device of a stat's Sys(), zeros for any other (API.md S1).
func sysStamp(sys any) (ctime int64, ino, dev uint64) {
	st, ok := sys.(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, 0, 0
	}
	return st.Ctimespec.Nano(), st.Ino, uint64(st.Dev)
}
