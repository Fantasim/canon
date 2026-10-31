package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// evalSymlinksOS is name's real, absolute, '/'-separated path on the OS's own disk (evalSymlinksWith).
func evalSymlinksOS(name string) (string, error) { return evalSymlinksWith(osLinks{}, name) }

// evalSymlinksWith is name's real path as lr reads it: a segment-by-segment walk, every link
// followed (readLink), so a cycle is ErrSymlinkLoop past MaxSymlinkHops, never the OS's own wording.
func evalSymlinksWith(lr linkReader, name string) (string, error) {
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
		link, isLink, err := readLink(lr, child)
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

// linkReader is what evalSymlinksWith reads of a disk, names and targets '/'-separated: an
// entry's Lstat mode, and the target of a link.
type linkReader interface {
	lstat(name string) (fs.FileMode, error)
	readlink(name string) (string, error)
}

// osLinks is the OS's own disk.
type osLinks struct{}

func (osLinks) lstat(name string) (fs.FileMode, error) {
	info, err := os.Lstat(filepath.FromSlash(name))
	if err != nil {
		return 0, fmt.Errorf(fmtWrap, err)
	}
	return info.Mode(), nil
}

func (osLinks) readlink(name string) (string, error) {
	raw, err := os.Readlink(filepath.FromSlash(name))
	if err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	return slashed(raw, filepath.Separator), nil
}

// readLink is child's target when it is a link, a symbolic link or a junction, a device target errDeviceTarget (WIRE.md §6.5, DECISIONS 419).
func readLink(lr linkReader, child string) (target string, isLink bool, err error) {
	mode, err := lr.lstat(child)
	if err != nil {
		return "", false, err
	}
	switch {
	case mode&fs.ModeSymlink != 0:
		if target, err = lr.readlink(child); err != nil {
			return "", false, err
		}
	case mode&fs.ModeIrregular == 0:
		return "", false, nil
	default:
		if target, isLink, err = irregularLink(lr, child); !isLink {
			return "", false, err
		}
	}
	if isDevicePath(target) {
		return "", false, errDeviceTarget
	}
	return target, true, nil
}

// irregularLink is the target of child, an irregular entry, when lr reads one; false with no error only for a non-link (notLinkErrs), as a cloud placeholder or an app-exec link.
func irregularLink(lr linkReader, child string) (string, bool, error) {
	target, err := lr.readlink(child)
	if isNotLink(err) {
		return "", false, nil
	}
	return target, err == nil, err
}

// isNotLink reports err readlink's answer for an entry that is no link (DECISIONS 419).
func isNotLink(err error) bool {
	return err != nil && slices.ContainsFunc(notLinkErrs[:], func(target error) bool { return errors.Is(err, target) })
}

// isDevicePath reports target a device path, `//?/…` or `//./…`, a volume mount point's included.
func isDevicePath(target string) bool {
	for _, prefix := range devicePrefixes {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
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
