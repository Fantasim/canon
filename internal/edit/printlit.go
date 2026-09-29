package edit

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/types"
)

// sourceText is an operation's value as Canon literal text (API.md E26), laid out by format.Flat:
// canonical, on one line. A FromJSON inside another value, a Float that is not finite or a Dur
// with a fraction of a millisecond has none: ErrNoText.
func sourceText(l Lit) (string, error) {
	var b strings.Builder
	if err := writeLit(&b, l); err != nil {
		return "", err
	}
	return flatText(b.String()), nil
}

// flatText is text re-printed by format.Flat when it parses as one literal, else as given.
func flatText(text string) string {
	e, f := parseLiteral(text)
	if e == nil {
		return text
	}
	out, err := format.Flat(f, e)
	if err != nil {
		return text
	}
	return string(out)
}

func writeLit(b *strings.Builder, l Lit) error {
	switch x := l.(type) {
	case List:
		return writeList(b, x)
	case Obj:
		return writeObj(b, x)
	case Map:
		return writeMap(b, x)
	case Case:
		b.WriteString(x.Name)
		if len(x.Fields) == 0 {
			return nil
		}
		b.WriteString(space)
		return writeObj(b, x.Fields)
	case Source:
		b.WriteString(string(x))
		return nil
	}
	text, ok := scalarText(l)
	if !ok {
		return ErrNoText
	}
	b.WriteString(text)
	return nil
}

// scalarText is a scalar Lit's literal: a key as a string or digits, a member as its name.
func scalarText(l Lit) (string, bool) {
	switch x := l.(type) {
	case Bool:
		return strconv.FormatBool(bool(x)), true
	case Int:
		return strconv.FormatInt(int64(x), decimalBase), true
	case IntKey:
		return strconv.FormatInt(int64(x), decimalBase), true
	case Float:
		f := float64(x)
		return types.FloatText(f, int64Bits), !math.IsNaN(f) && !math.IsInf(f, 0)
	case Str:
		return canonQuote(string(x)), true
	case Key:
		return canonQuote(string(x)), true
	case PathKey:
		if isWord(string(x)) {
			return string(x), true
		}
		return canonQuote(string(x)), true
	case Member:
		return string(x), true
	case None:
		return noneWord, true
	case Dur:
		d := time.Duration(x)
		return types.DurationText(d.Milliseconds()), d%time.Millisecond == 0
	}
	return "", false
}

func writeList(b *strings.Builder, l List) error {
	b.WriteString(string(bracketOpen))
	for i, e := range l {
		if i > 0 {
			b.WriteString(listSep)
		}
		if err := writeLit(b, e); err != nil {
			return err
		}
	}
	b.WriteString(bracketClose)
	return nil
}

// writeObj writes an Obj's fields by name, which a record literal may give in any order.
func writeObj(b *strings.Builder, o Obj) error {
	if len(o) == 0 {
		b.WriteString(emptyBrace)
		return nil
	}
	b.WriteString(braceOpenSp)
	for i, name := range slices.Sorted(maps.Keys(o)) {
		if i > 0 {
			b.WriteString(listSep)
		}
		b.WriteString(name + colonSp)
		if err := writeLit(b, o[name]); err != nil {
			return err
		}
	}
	b.WriteString(braceCloseSp)
	return nil
}

func writeMap(b *strings.Builder, m Map) error {
	if len(m) == 0 {
		b.WriteString(emptyBrace)
		return nil
	}
	b.WriteString(braceOpenSp)
	for i, kv := range m {
		if i > 0 {
			b.WriteString(listSep)
		}
		if err := writeLit(b, kv.Key); err != nil {
			return err
		}
		b.WriteString(colonSp)
		if err := writeLit(b, kv.Value); err != nil {
			return err
		}
	}
	b.WriteString(braceCloseSp)
	return nil
}
