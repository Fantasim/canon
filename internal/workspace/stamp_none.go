//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || illumos)

package workspace

// sysStamp has no change time or identity on this platform: zeros (API.md S1).
func sysStamp(any) (ctime int64, ino, dev uint64) { return 0, 0, 0 }
