package rules

import (
	"regexp"
	"strings"
)

var (
	historyRE = regexp.MustCompile(historyPattern)
	presentRE = regexp.MustCompile(presentPrevious)
)

// NarratesHistory reports whether comment text describes a past change instead of the
// present code.
func NarratesHistory(text string) bool {
	if historyRE.MatchString(presentRE.ReplaceAllString(text, "")) {
		return true
	}
	for _, m := range usedTo.FindAllStringSubmatch(text, -1) {
		prev := strings.ToLower(strings.Trim(m[1], wordPunct))
		if prev != "" && !passiveBefore[prev] && !strings.HasSuffix(m[1], ".") {
			return true
		}
	}
	return false
}
