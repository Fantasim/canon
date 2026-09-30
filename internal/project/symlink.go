package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// evalSymlinksOS is name's real, absolute, '/'-separated path: osFS's own segment-by-segment walk, every link followed, so a cycle is ErrSymlinkLoop past MaxSymlinkHops, never the OS's own wording.
func evalSymlinksOS(name string) (string, error) {
	vol := volumeOf(name)
	dest, pending, hops := vol, segmentsOf(strings.TrimPrefix(name, vol)), 0
	for len(pending) > 0 {
		seg := pending[0]
		pending = pending[1:]
		if next, ok := applyDotSegment(dest, seg); ok {
			dest = next
			continue
		}
		child := dest + sep + seg
		link, isLink, err := readLink(child)
		switch {
		case err != nil:
			return "", err
		case !isLink:
			dest = child
			continue
		}
		if hops++; hops > MaxSymlinkHops {
			return "", fmt.Errorf("%w: %s", ErrSymlinkLoop, name)
		}
		dest, pending = followLink(dest, link, pending)
	}
	if dest == volumeOf(dest) {
		return dest + sep, nil
	}
	return dest, nil
}

// volumeOf is name's leading volume, a drive letter or a UNC share, "" for neither (a device path `//?/…` or `//./…` is neither): parsed as text, never through the OS, so it is the same on every GOOS.
func volumeOf(name string) string {
	if drivePattern.MatchString(name) {
		return name[:driveLen]
	}
	rest, ok := strings.CutPrefix(name, uncPrefix)
	if !ok {
		return ""
	}
	host, tail, ok := strings.Cut(rest, sep)
	if !ok || host == "" || host == queryHost || host == dot {
		return ""
	}
	share, _, _ := strings.Cut(tail, sep)
	if share == "" {
		return ""
	}
	return uncPrefix + host + sep + share
}

// segmentsOf is name split on '/', its leading and trailing ones dropped.
func segmentsOf(name string) []string {
	trimmed := strings.Trim(name, sep)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, sep)
}

// applyDotSegment handles "" and "." (dropped) and ".." (dest backed up one segment, or kept at
// its volume's root); ok is false for an ordinary segment, left to the caller.
func applyDotSegment(dest, seg string) (string, bool) {
	switch seg {
	case "", currentSeg:
		return dest, true
	case parentSeg:
		vol := volumeOf(dest)
		if i := strings.LastIndexByte(dest, '/'); i >= len(vol) {
			return dest[:i], true
		}
		return vol, true
	default:
		return "", false
	}
}

// readLink is child's Lstat and, when it is a symbolic link, its raw target text, '/'-separated.
func readLink(child string) (target string, isLink bool, err error) {
	info, err := os.Lstat(filepath.FromSlash(child))
	if err != nil {
		return "", false, fmt.Errorf(fmtWrap, err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return "", false, nil
	}
	raw, err := os.Readlink(filepath.FromSlash(child))
	if err != nil {
		return "", false, fmt.Errorf(fmtWrap, err)
	}
	return slashed(raw, filepath.Separator), true, nil
}

// followLink is the walk's state once child, dest's last segment, resolves to link: an absolute
// target naming its own volume restarts there, one without stays on dest's current volume, a
// relative one stays under dest, its own directory.
func followLink(dest, link string, pending []string) (string, []string) {
	vol := volumeOf(link)
	next := append(segmentsOf(strings.TrimPrefix(link, vol)), pending...)
	switch {
	case vol != "":
		return vol, next
	case isAbsolute(link):
		return volumeOf(dest), next
	default:
		return dest, next
	}
}

// slashed is p, an OS path whose separator is osSep, '/'-separated as FS names are (API.md §2.2).
func slashed(p string, osSep rune) string {
	return strings.ReplaceAll(p, string(osSep), sep)
}
