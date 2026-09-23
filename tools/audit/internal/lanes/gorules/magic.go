package gorules

import (
	"fmt"
	"go/constant"
	"go/token"
	"path"
	"slices"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// occurrence is one judged string literal.
type occurrence struct {
	file *src
	line int
}

// magicStrings counts judged string values repo-wide and reports each repeated value once
// per package that uses it.
func (s *scan) magicStrings() {
	for v, dirs := range s.stringUses() {
		total := 0
		for _, occ := range dirs {
			total += len(occ)
		}
		if total >= rules.MagicStringMin {
			s.magicValue(v, dirs, total)
		}
	}
}

// stringUses maps each judged, non-trivial string value of non-test files to its
// occurrences by package directory, in file and position order.
func (s *scan) stringUses() map[string]map[string][]occurrence {
	byVal := map[string]map[string][]occurrence{}
	for _, f := range s.files {
		for _, j := range f.lits {
			v, ok := unquote(j.lit)
			if f.TestCode() || j.quietStr || !ok || trivialString(v) {
				continue
			}
			if byVal[v] == nil {
				byVal[v] = map[string][]occurrence{}
			}
			byVal[v][f.Dir] = append(byVal[v][f.Dir], occurrence{file: f, line: s.line(j.lit.Pos())})
		}
	}
	return byVal
}

func (s *scan) magicValue(v string, dirs map[string][]occurrence, total int) {
	for dir, occ := range dirs {
		first := occ[0]
		fix := s.magicFix(v, len(dirs), first.file)
		msg := fmt.Sprintf(msgMagicString, clip(v, snippetMax), len(occ), first.file.base(), first.line)
		if len(dirs) > 1 {
			msg = fmt.Sprintf(msgMagicShared, clip(v, snippetMax), len(occ), total, len(dirs), first.file.base(), first.line)
		}
		f := finding.Finding{
			Rule: ruleMagicString, File: dir, Line: first.line, Detail: clip(finding.Clean(v), detailMax),
			Value: len(occ), Message: msg, Fix: fix, Src: first.file.Path,
		}
		if f.Fix == "" {
			f.Fix = fmt.Sprintf(fixConstIn, path.Join(dir, constantsFile))
		}
		s.emit(f)
	}
}

func (s *scan) magicFix(v string, pkgs int, f *src) string {
	if std, ok := f.stdFix(v); ok {
		return fmt.Sprintf(fixUse, std)
	}
	if pkgs == 1 {
		return ""
	}
	return fixConstShared
}

// trivialString is a value no one would name: one rune, or only punctuation around fmt verbs.
func trivialString(v string) bool {
	if utf8.RuneCountInString(v) <= singleRune {
		return true
	}
	rest := fmtVerb.ReplaceAllString(v, "")
	return !slices.ContainsFunc([]rune(rest), func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
}

// magicNumbers reports every judged number other than 0 and 1, outside constants.go.
func (s *scan) magicNumbers() {
	for _, f := range s.files {
		if f.TestCode() || f.base() == constantsFile {
			continue
		}
		for _, j := range f.lits {
			if !numeric(j.lit.Kind) || j.quietNum || plainNumber(j.lit.Value, j.lit.Kind) {
				continue
			}
			ctx := j.lit.Value
			if j.parent != nil {
				ctx = clip(s.source(f, j.parent), snippetMax)
			}
			s.emit(finding.Finding{
				Rule: ruleMagicNumber, File: f.Path, Line: s.line(j.lit.Pos()), Detail: j.lit.Value,
				Message: fmt.Sprintf(msgMagicNumber, j.lit.Value, strconv.Quote(ctx)),
				Fix:     fmt.Sprintf(fixConstIn, path.Join(f.Dir, constantsFile)),
			})
		}
	}
}

func numeric(k token.Token) bool { return k == token.INT || k == token.FLOAT || k == token.IMAG }

// plainNumber is 0 or 1: an empty count, a first index or a step, never worth a name.
func plainNumber(lit string, kind token.Token) bool {
	return slices.ContainsFunc(plainNumbers, func(n int64) bool { return litIs(lit, kind, n) })
}

// litIs reports a numeric literal equal to n.
func litIs(lit string, kind token.Token, n int64) bool {
	v := constant.MakeFromLiteral(lit, kind, 0)
	return v.Kind() != constant.Unknown && constant.Compare(v, token.EQL, constant.MakeInt64(n))
}
