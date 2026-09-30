package winpaths

import "strings"

// VolumeName is filepath.VolumeName on Windows: name's volume with every '/' written '\'.
func VolumeName(name string) string {
	return strings.ReplaceAll(name[:volumeNameLen(name)], slash, Sep)
}

func isSep(c byte) bool { return c == backslashCh || c == '/' }

// volumeNameLen is the length of name's leading volume: a drive, a UNC share (or as much of one
// as is written), or a device path's first component.
func volumeNameLen(name string) int {
	switch {
	case len(name) >= driveLen && name[1] == colon:
		return driveLen
	case len(name) == 0 || !isSep(name[0]):
		return 0
	case hasPrefixFold(name, uncDevice):
		return uncLen(name, uncDeviceEnd)
	case hasPrefixFold(name, devicePrefix) || hasPrefixFold(name, rootDevice) || hasPrefixFold(name, ntPrefix):
		return deviceLen(name)
	case len(name) >= uncPrefixLen && isSep(name[1]):
		return uncLen(name, uncPrefixLen)
	}
	return 0
}

// deviceLen is the length of a device path's volume: the prefix and the next component.
func deviceLen(name string) int {
	if len(name) == deviceOnly {
		return deviceOnly
	}
	for i := deviceSkip; i < len(name); i++ {
		if isSep(name[i]) {
			return i
		}
	}
	return len(name)
}

// hasPrefixFold reports name starting with prefix, letter case and separator kind ignored, and
// then ending or reaching a separator.
func hasPrefixFold(name, prefix string) bool {
	if len(name) < len(prefix) || !strings.EqualFold(strings.ReplaceAll(name[:len(prefix)], slash, Sep), prefix) {
		return false
	}
	return len(name) == len(prefix) || isSep(name[len(prefix)])
}

// uncLen is the length of a UNC path's volume: up to the second separator after the host starts.
func uncLen(name string, start int) int {
	seen := 0
	for i := start; i < len(name); i++ {
		if !isSep(name[i]) {
			continue
		}
		if seen++; seen == uncSeps {
			return i
		}
	}
	return len(name)
}
