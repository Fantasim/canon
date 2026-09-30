package project

import (
	"path"
	"path/filepath"
	"strings"
)

// Paths is a system's separator, read as '/', and volume parser (API.md §2.2, log M4 B12-r).
type Paths struct {
	Sep    string
	Volume func(string) string
}

// HostPaths is the running system's: filepath.Separator and filepath.VolumeName.
func HostPaths() Paths {
	return Paths{Sep: string(filepath.Separator), Volume: filepath.VolumeName}
}

// Clean is path.Clean after name's host volume (Paths.clean).
func Clean(name string) string { return HostPaths().clean(name) }

// Join is HostPaths().Join(dir, elem...).
func Join(dir string, elem ...string) string { return HostPaths().Join(dir, elem...) }

// DirOf is path.Dir after name's host volume (Paths.dir).
func DirOf(name string) string { return HostPaths().dir(name) }

// FromAPI is name, given to the API, as an FS name: each Sep a '/'; a rooted name without a
// volume put on dir's, then cleaned (clean). A relative name is left to its caller.
func (s Paths) FromAPI(name, dir string) string {
	name = strings.ReplaceAll(name, s.Sep, sep)
	vol := s.Volume(name)
	if !path.IsAbs(name[len(vol):]) {
		return name
	}
	if vol == "" {
		name = s.Volume(dir) + name
	}
	return s.clean(name)
}

// clean is path.Clean after name's volume, a drive letter upper-cased: path.Clean alone turns
// "C:/.." into "." and "//host/share/x" into "/host/share/x". A bare volume is its root.
func (s Paths) clean(name string) string {
	return s.afterVolume(name, path.Clean)
}

// Join is path.Join of elem under dir, dir's volume kept.
func (s Paths) Join(dir string, elem ...string) string {
	if s.Volume(dir) == "" {
		return path.Join(append([]string{dir}, elem...)...)
	}
	return s.clean(dir + sep + path.Join(elem...))
}

// dir is path.Dir after name's volume: a volume's root is its own parent.
func (s Paths) dir(name string) string {
	return s.afterVolume(name, path.Dir)
}

// afterVolume applies op to name after its volume, the volume's drive letter upper-cased.
func (s Paths) afterVolume(name string, op func(string) string) string {
	vol := s.Volume(name)
	if vol == "" {
		return op(name)
	}
	rest := name[len(vol):]
	if rest == "" {
		rest = sep
	}
	if len(vol) == driveLen && drivePattern.MatchString(vol) {
		vol = strings.ToUpper(vol)
	}
	return vol + op(rest)
}
