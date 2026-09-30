package load

import (
	"crypto/sha256"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// taken is how a load took a file into the set: its bytes when it read them, else where to read
// them from.
type taken struct {
	l    *Loader
	abs  string
	data []byte
	read bool
}

// raw is the file's bytes as written: those read, or read now, which within one snapshot is the
// content its SHA-256 is of (log-2026-09-29 P18).
func (t taken) raw() []byte {
	if t.read {
		return t.data
	}
	// The error is nil: FS read this same snapshot entry for the sum Kept matched, without error.
	data, _ := t.l.FS.ReadFile(t.abs)
	return data
}

// takeSource is readSource for a load that needs a file's bytes only to report a finding: a file
// whose SHA-256 the FS knows is taken unread when Loader.Kept, a cache, has that content, and is
// recorded as readSource records it (log-2026-09-29 P18).
func (l *Loader) takeSource(display, abs string, req Request) (*source.File, taken, bool) {
	if sum, known := l.fixedSum(abs); known {
		if src, ok := l.Kept(display, abs, sum); ok {
			if l.rec != nil {
				l.note(callSource, abs, display, func() answer { return sumAnswer(sum, nil) })
			}
			return src, taken{l: l, abs: abs}, true
		}
	}
	src, data, ok := l.readSource(display, abs, req)
	return src, taken{data: data, read: true}, ok
}

// fixedSum is the SHA-256 the FS knows of abs, false when it knows none, the read fails or no
// Kept could take the content: the caller then reads abs.
func (l *Loader) fixedSum(abs string) ([sha256.Size]byte, bool) {
	if l.Kept == nil {
		return [sha256.Size]byte{}, false
	}
	sum, ok, err := project.SumFile(l.FS, abs)
	return sum, ok && err == nil
}
