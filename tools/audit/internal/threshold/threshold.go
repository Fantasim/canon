package threshold

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Set is every rule limit thresholds.tsv holds, one field per key.
type Set struct {
	FnLines, FnStatements, TestFnLines, FnParams, FnResults, FnComplexity, FnNesting int
	NakedReturnLines, FileLines, PkgFiles, PkgLines, MagicStringUses                 int
	CommentDeclLines, CommentPkgLines, CommentBlockLines, CommentRatioPercent        int
	CommentRatioMinLines, CommentFieldLines, DupTokens, DiagTextWords                int
}

// spec is one key: where its value lives in Set and its upper bound.
type spec struct {
	key   string
	field func(*Set) *int
	max   int
}

// Raise is one threshold whose value went up against an earlier version of the file.
type Raise struct {
	Key      string
	Was, Now int
}

// Load reads and strictly parses the thresholds file at path.
func Load(path string) (Set, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Set{}, fmt.Errorf("%w: %w", errRead, err)
	}
	return parse(string(data), path)
}

// parse accepts only comment lines, blank lines and one valid row per known key, and
// reports every problem of the file at once.
func parse(text, name string) (Set, error) {
	var (
		s    Set
		errs []error
	)
	seen := map[string]bool{}
	for i, line := range strings.Split(text, lineBreak) {
		if line == "" || strings.HasPrefix(line, commentMark) {
			continue
		}
		sp, v, err := parseRow(line, seen)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s:%d: %w", name, i+1, err))
			continue
		}
		*sp.field(&s) = v
	}
	for _, sp := range specs {
		if !seen[sp.key] {
			errs = append(errs, fmt.Errorf("%s: %w: %s", name, errMissing, sp.key))
		}
	}
	return s, errors.Join(errs...)
}

// parseRow checks one data row against the known keys and the keys already seen.
func parseRow(line string, seen map[string]bool) (spec, int, error) {
	cells := strings.Split(line, fieldSep)
	if len(cells) != rowFields {
		return spec{}, 0, fmt.Errorf("%w: %q", errMalformed, line)
	}
	key, raw := cells[0], cells[1]
	i := slices.IndexFunc(specs, func(sp spec) bool { return sp.key == key })
	if i < 0 {
		return spec{}, 0, fmt.Errorf("%w: %q", errUnknown, key)
	}
	if seen[key] {
		return spec{}, 0, fmt.Errorf("%w: %s", errDuplicate, key)
	}
	seen[key] = true
	v, err := strconv.Atoi(raw)
	if !reValue.MatchString(raw) || err != nil || specs[i].max != noMax && v > specs[i].max {
		return spec{}, 0, fmt.Errorf("%w: %s %q", errValue, key, raw)
	}
	return specs[i], v, nil
}

// Expand replaces every `{key}` in text by that threshold's value; an unknown key is an error.
func (s Set) Expand(text string) (string, error) {
	var unknown []string
	out := rePlaceholder.ReplaceAllStringFunc(text, func(m string) string {
		key := rePlaceholder.FindStringSubmatch(m)[placeholderGroup]
		if v, ok := s.value(key); ok {
			return strconv.Itoa(v)
		}
		unknown = append(unknown, key)
		return m
	})
	if len(unknown) > 0 {
		return text, fmt.Errorf("%w: %s", errUnknown, strings.Join(unknown, ", "))
	}
	return out, nil
}

func (s Set) value(key string) (int, bool) {
	for _, sp := range specs {
		if sp.key == key {
			return *sp.field(&s), true
		}
	}
	return 0, false
}

// Rows reads the well-formed rows of a thresholds file of any version, ignoring the rest:
// the base revision's file is compared, never trusted to parse under today's keys.
func Rows(text string) map[string]int {
	out := map[string]int{}
	for line := range strings.SplitSeq(text, lineBreak) {
		cells := strings.Split(line, fieldSep)
		if len(cells) != rowFields || strings.HasPrefix(line, commentMark) || !reValue.MatchString(cells[1]) {
			continue
		}
		if v, err := strconv.Atoi(cells[1]); err == nil {
			out[cells[0]] = v
		}
	}
	return out
}

// Raised lists, in file order, every key whose value in s is above its value in prev.
func (s Set) Raised(prev map[string]int) []Raise {
	var out []Raise
	for _, sp := range specs {
		now := *sp.field(&s)
		if was, ok := prev[sp.key]; ok && now > was {
			out = append(out, Raise{Key: sp.key, Was: was, Now: now})
		}
	}
	return out
}
