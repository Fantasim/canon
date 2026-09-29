package edit

import (
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// key types the operation's key as t, text that is not UTF-8 refused (GRAMMAR.md §1).
func (x *opCtx) key(t types.Type) (value.Value, error) {
	if err := utf8Text(x.op.Key, t); err != nil {
		return nil, err
	}
	return x.a.typer(x.res.root.pkg.Path).Key(x.a.ctx, x.op.Key, t)
}

// utf8Text refuses lit holding text that is not UTF-8, which no source holds (GRAMMAR.md §1).
func utf8Text(lit Lit, t types.Type) error {
	if validText(lit) {
		return nil
	}
	return &ValueError{Expected: t.String(), Got: describe(lit), Detail: detailNotUTF8}
}

// validText reports every text lit holds, its own and its parts', valid UTF-8.
func validText(lit Lit) bool {
	switch x := lit.(type) {
	case Str:
		return utf8.ValidString(string(x))
	case Key:
		return utf8.ValidString(string(x))
	case PathKey:
		return utf8.ValidString(string(x))
	case Member:
		return utf8.ValidString(string(x))
	case Source:
		return utf8.ValidString(string(x))
	case FromJSON:
		return utf8.Valid(x)
	case List:
		return allValid(x)
	case Obj:
		return validObj(x)
	case Case:
		return utf8.ValidString(x.Name) && validObj(x.Fields)
	case Map:
		for _, kv := range x {
			if !validText(kv.Key) || !validText(kv.Value) {
				return false
			}
		}
	}
	return true
}

func allValid(ls []Lit) bool {
	for _, l := range ls {
		if !validText(l) {
			return false
		}
	}
	return true
}

func validObj(o Obj) bool {
	for name, l := range o { //canon:unordered an all-of test
		if !utf8.ValidString(name) || !validText(l) {
			return false
		}
	}
	return true
}
