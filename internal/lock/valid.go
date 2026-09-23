package lock

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// valid checks that fact's line reads back as fact: the shape of its kind, a name in the
// lock's package, identifiers, and a string that is valid UTF-8.
func (f *File) valid(fact Fact) error {
	if int(fact.Kind) >= len(kindNames) {
		return fmt.Errorf(fmtBadFact, ErrBadFact, errKind)
	}
	own := strings.Split(f.Package, nameSep)
	for _, check := range factChecks {
		if reason := check(own, fact); reason != nil {
			return fmt.Errorf(fmtBadFactLine, ErrBadFact, reason, fact.line())
		}
	}
	return nil
}

// factChecks are what valid checks, in order; each returns the reason a fact fails, or nil.
var factChecks = []func(own []string, fact Fact) error{
	checkName,
	checkField,
	checkValue,
	checkHolder,
	checkRetired,
}

// checkName: the table's let or the enum, `<package>.<identifier>`; own is the lock's package.
func checkName(own []string, fact Fact) error {
	segs := strings.Split(fact.Name, nameSep)
	if len(segs) != len(own)+1 || !slices.Equal(segs[:len(own)], own) ||
		slices.ContainsFunc(segs, func(s string) bool { return !isIdentifier(s) }) {
		return errName
	}
	return nil
}

// checkField: a field fact names its field, the other kinds none.
func checkField(_ []string, fact Fact) error {
	if (fact.Kind == KindField) != (fact.Field != "") || fact.Field != "" && !isIdentifier(fact.Field) {
		return errField
	}
	return nil
}

// checkValue: a table fact has no value, an enum fact an integer, a field fact an integer or
// a string of valid UTF-8; the part of Value the kind does not write is zero.
func checkValue(_ []string, fact Fact) error {
	v := fact.Value
	switch {
	case fact.Kind == KindTable && v != Value{}:
		return errTableValue
	case fact.Kind == KindEnum && v.IsString:
		return errEnumString
	case v.IsString && (v.Int != 0 || !utf8.ValidString(v.Str)):
		return errString
	case !v.IsString && v.Str != "":
		return errString
	}
	return nil
}

func checkHolder(_ []string, fact Fact) error {
	if !isIdentifier(fact.Holder) {
		return errHolder
	}
	return nil
}

// checkRetired: a field fact has no retirement of its own (LOCK.md §2.2).
func checkRetired(_ []string, fact Fact) error {
	if fact.Kind == KindField && fact.Retired {
		return errFieldRetired
	}
	return nil
}

// line is the fact as Format would write it, for error texts.
func (fact Fact) line() string {
	return string(fact.appendLine(nil))
}
