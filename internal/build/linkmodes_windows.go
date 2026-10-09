package build

import "io/fs"

// linkEntry reports a listing's entry type a build never writes through: a symbolic link, or a
// junction or other reparse point, which Go lists as irregular (DECISIONS 342).
func linkEntry(t fs.FileMode) bool {
	return t&(fs.ModeSymlink|fs.ModeIrregular) != 0
}
