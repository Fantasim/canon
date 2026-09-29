//go:build linux

package main

import (
	"encoding/json"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/api/vm"
)

// A text value's type, as the view model encodes it (TypeInfo.VM, VIEWMODEL.md type encoding).
const (
	vmKindAsset    = "asset"
	vmKindUnion    = "union"
	vmKindString   = "string"
	vmKindOptional = "optional"
	patternMark    = "/"
	extSep         = ","
	digitsFrom     = '1' // a digit drawn is 1-9: a pattern such as [1-9] still matches
	digitsCount    = 9
	lettersCount   = 26
	coinSides      = 2
)

// vmType is v's type in the view model's encoding, an optional unwrapped.
func vmType(v *canon.Value) vm.TypeExpr {
	var te vm.TypeExpr
	if err := json.Unmarshal(v.Type.VM, &te); err != nil {
		return vm.TypeExpr{}
	}
	for te.Kind == vmKindOptional && te.Of != nil {
		te = *te.Of
	}
	return te
}

// fieldKind is the kind a draw picks among: the value's own, a text split by its type into
// string, asset and union (log-2026-09-29 M4 U7b-r3: every editable scalar kind is drawn).
func fieldKind(v *canon.Value) string {
	if v.Kind != canon.KindString && v.Kind != canon.KindAsset {
		return string(v.Kind)
	}
	switch te := vmType(v); te.Kind {
	case vmKindAsset, vmKindUnion:
		return te.Kind
	}
	return string(canon.KindString)
}

// freshText draws a text by its type: another file of an asset's root, another literal of a
// union, else the string changed within its refinement.
func freshText(f *freshValues, v *canon.Value) (canon.Lit, any, bool) {
	switch te := vmType(v); te.Kind {
	case vmKindAsset:
		return f.freshAsset(te)
	case vmKindUnion:
		return f.freshUnion(v, te)
	default:
		return f.freshString(v, te)
	}
}

// freshAsset is another existing file under the asset's root with one of its extensions.
func (f *freshValues) freshAsset(te vm.TypeExpr) (canon.Lit, any, bool) {
	files := f.assetFiles(te)
	if len(files) == 0 {
		return nil, nil, false
	}
	name := files[f.rng.IntN(len(files))]
	return canon.Str(name), name, true
}

// assetFiles are the files of an asset root, root-relative with '/', sorted, read once.
func (f *freshValues) assetFiles(te vm.TypeExpr) []string {
	key := te.Root + patternMark + strings.Join(te.Ext, extSep)
	if files, ok := f.files[key]; ok {
		return files
	}
	var files []string
	if dir, ok := f.assets[te.Root]; ok {
		root := filepath.Join(f.dir, filepath.FromSlash(dir))
		_ = filepath.WalkDir(root, func(name string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && extAllowed(name, te.Ext) {
				if rel, err := filepath.Rel(root, name); err == nil {
					files = append(files, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	slices.Sort(files)
	f.files[key] = files
	return files
}

// extAllowed is whether a file has one of the extensions, given without dots; any for none.
func extAllowed(name string, exts []string) bool {
	return len(exts) == 0 || slices.Contains(exts, strings.TrimPrefix(path.Ext(filepath.ToSlash(name)), "."))
}

// freshUnion is another literal of the union, or, one draw in two when its base is a string,
// the string changed.
func (f *freshValues) freshUnion(v *canon.Value, te vm.TypeExpr) (canon.Lit, any, bool) {
	if te.Of != nil && te.Of.Kind == vmKindString && (len(te.Literals) == 0 || f.rng.IntN(coinSides) == 0) {
		return f.freshString(v, *te.Of)
	}
	if len(te.Literals) == 0 {
		return nil, nil, false
	}
	lit := te.Literals[f.rng.IntN(len(te.Literals))]
	return canon.Str(lit), lit, true
}

// freshString changes one digit of the string (to 1-9), else one letter (to one of its case),
// and keeps the result only if it still matches the type's pattern: a new unused code for an
// id such as ITM_000123, a new word for a name; its length, and so its bounds, are kept.
func (f *freshValues) freshString(v *canon.Value, te vm.TypeExpr) (canon.Lit, any, bool) {
	s, _ := v.Str()
	r := []rune(s)
	at := positions(r, unicode.IsDigit)
	if len(at) == 0 {
		at = positions(r, unicode.IsLetter)
	}
	if len(at) == 0 {
		return nil, nil, false
	}
	i := at[f.rng.IntN(len(at))]
	switch {
	case unicode.IsDigit(r[i]):
		r[i] = digitsFrom + rune(f.rng.IntN(digitsCount))
	case unicode.IsUpper(r[i]):
		r[i] = 'A' + rune(f.rng.IntN(lettersCount))
	default:
		r[i] = 'a' + rune(f.rng.IntN(lettersCount))
	}
	out := string(r)
	if re := pattern(te, v.Type.Expr); re != nil && !re.MatchString(out) {
		return nil, nil, false
	}
	return canon.Str(out), out, true
}

func positions(r []rune, is func(rune) bool) []int {
	var out []int
	for i, c := range r {
		if is(c) {
			out = append(out, i)
		}
	}
	return out
}

// pattern is a string type's pattern refinement: the view model's, else the one written in the
// type text (String(/.../)); nil for none.
func pattern(te vm.TypeExpr, expr string) *regexp.Regexp {
	src := ""
	if te.Pattern != nil {
		src = *te.Pattern
	} else if lo, hi := strings.Index(expr, patternMark), strings.LastIndex(expr, patternMark); lo >= 0 && hi > lo {
		src = expr[lo+1 : hi]
	}
	if src == "" {
		return nil
	}
	re, err := regexp.Compile(src)
	if err != nil {
		return nil
	}
	return re
}
