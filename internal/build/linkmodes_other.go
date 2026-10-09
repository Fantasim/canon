//go:build !windows

package build

import "io/fs"

// linkEntry reports a listing's entry type a build never writes through: a symbolic link (DECISIONS 342).
func linkEntry(t fs.FileMode) bool {
	return t&fs.ModeSymlink != 0
}
