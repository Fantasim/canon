package edit

import (
	"crypto/sha256"

	"github.com/fantasim/canonlang/internal/syntax"
)

// VerdictKey is a text's hash and whether it parses as a project file: all M9's verdict depends on.
type VerdictKey struct {
	Sum     [sha256.Size]byte
	Project bool
}

// Verdicts is the caller's bounded, concurrency-safe memo of the texts M9 found fixed (API.md M9).
type Verdicts interface {
	Fixed(VerdictKey) bool
	Keep(VerdictKey)
}

// verdictKey is the key of raw parsed as kind.
func verdictKey(raw []byte, kind syntax.FileKind) VerdictKey {
	return VerdictKey{Sum: sha256.Sum256(raw), Project: kind == syntax.FileProject}
}

// fixed reports a kept verdict that raw, parsed as kind, is a fixed point (API.md M9).
func (a *applier) fixed(key VerdictKey) bool {
	return a.env.Verdicts != nil && a.env.Verdicts.Fixed(key)
}

// keep records that the text of key is a fixed point.
func (a *applier) keep(key VerdictKey) {
	if a.env.Verdicts != nil {
		a.env.Verdicts.Keep(key)
	}
}
