package load

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// DirGlob is the literal base and the glob of e, a load.dir of a literal pattern with a glob: CODEGEN.md §2.4, DECISIONS 451.
func DirGlob(e *syntax.LoadExpr) (base, rest string, ok bool) {
	if e.Method == nil || e.Method.Name != methodDir {
		return "", "", false
	}
	pattern, ok := literalPath(e)
	if !ok || strings.Contains(pattern, backslash) {
		return "", "", false
	}
	base, rest = splitGlob(pattern)
	if rest == "" {
		return "", "", false
	}
	return base, rest, true
}

// MatchesPlanned reports whether the load.dir pattern selects display, a path no file has yet: CODEGEN.md §2.4, DECISIONS 451.
func MatchesPlanned(pattern, display string) bool {
	prefix, rest := splitGlob(pattern)
	if rest == "" || strings.Contains(pattern, backslash) {
		return false
	}
	if chk := validateGlob(rest); chk.verdict != globWellFormed || chk.dirOnly {
		return false
	}
	below, ok := strings.CutPrefix(display, prefix)
	if !ok || below == "" {
		return false
	}
	return matchNames(mergeStars(strings.Split(rest, sepStr)), strings.Split(below, sepStr))
}

// matchNames is whether the file at the end of names, the path below the glob's base, is a match of segs.
func matchNames(segs, names []string) bool {
	if len(segs) == 0 || len(names) == 0 {
		return false
	}
	if segs[0] != doubleStar {
		if !segMatches(segs[0], names[0]) {
			return false
		}
		if len(segs) == 1 {
			return len(names) == 1
		}
		return matchNames(segs[1:], names[1:])
	}
	return matchStars(segs[1:], names)
}

// matchStars is whether a leading "**" followed by rest matches names: zero directories or more, none
// a dot-directory, then rest; at the end of the glob, every name is one a walk would enter.
func matchStars(rest, names []string) bool {
	if len(rest) == 0 {
		return noDot(names)
	}
	for k := range names {
		if matchNames(rest, names[k:]) {
			return true
		}
		if strings.HasPrefix(names[k], dotPrefix) {
			return false
		}
	}
	return false
}

// noDot is whether no name starts with a dot: a walk of "**" enters no such entry.
func noDot(names []string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, dotPrefix) {
			return false
		}
	}
	return true
}
