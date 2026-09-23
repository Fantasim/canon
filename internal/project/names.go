package project

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// IsIdent reports whether s is an IDENT: not a reserved word, not "_" (GRAMMAR.md §2.3).
func IsIdent(s string) bool {
	return identPattern.MatchString(s) && syntax.LookupWord(s) == syntax.TokIdent
}

// IsPackageName reports a dotted path of lowerCamel IDENTs (SPEC §3.2, GRAMMAR.md §9.2).
func IsPackageName(name string) bool {
	for seg := range strings.SplitSeq(name, nameSep) {
		if !IsIdent(seg) || !segmentPattern.MatchString(seg) {
			return false
		}
	}
	return true
}
