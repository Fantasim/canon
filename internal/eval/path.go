package eval

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// vpath is the value path a top-level value's literal builds, nil for none (EVALUATION.md §13).
type vpath struct {
	parent *vpath
	seg    string
	keyed  *types.Field // a keyed-list element still to be named: its key field (API.md P1, P8)
}

func rootPath(name string) *vpath {
	return &vpath{seg: name}
}

func (p *vpath) add(seg string) *vpath {
	if p == nil {
		return nil
	}
	return &vpath{parent: p, seg: seg}
}

func (p *vpath) field(name string) *vpath {
	return p.add(dot + name)
}

func (p *vpath) index(i int) *vpath {
	return p.add(keyOpen + strconv.Itoa(i) + keyClose)
}

// key is the path of a keyed-list element or map entry: a word bare, an integer in decimal,
// any other key as a JSON string (API.md P8, P9).
func (p *vpath) key(k value.Key) *vpath {
	return p.add(keyOpen + keyText(k) + keyClose)
}

// entry is a table entry: `.key`, or `["key"]` when the key is not a word.
func (p *vpath) entry(k value.Key) *vpath {
	if !k.IsInt && word(k.S) {
		return p.field(k.S)
	}
	return p.key(k)
}

// element is the path of a list's element i: its key once known when the list is keyed (API.md
// P1, P8), so until then, and for a key that never is, the list's own path.
func (p *vpath) element(lt *types.ListType, i int) *vpath {
	if p == nil || lt == nil || lt.KeyedBy == nil {
		return p.index(i)
	}
	return &vpath{parent: p, keyed: lt.KeyedBy}
}

// learn names a pending keyed-list element by the key v, the value of field f, gives it.
func (p *vpath) learn(f *types.Field, v value.Value) {
	if p == nil || p.keyed == nil || p.keyed != f || p.seg != "" {
		return
	}
	if k, ok := std.KeyOf(v); ok {
		p.seg = keyOpen + keyText(k) + keyClose
	}
}

// learnFrom is learn for an element built whole: its key field's value.
func (p *vpath) learnFrom(el value.Value) {
	if p == nil || p.keyed == nil {
		return
	}
	if rec, ok := el.(*value.Record); ok && p.keyed.Index < len(rec.Fields) && rec.Fields[p.keyed.Index] != nil {
		p.learn(p.keyed, rec.Fields[p.keyed.Index])
	}
}

func (p *vpath) String() string {
	var segs []string
	for q := p; q != nil; q = q.parent {
		if q.keyed != nil && q.seg == "" {
			segs = segs[:0] // below an element whose key is not known: the list's own path
		}
		segs = append(segs, q.seg)
	}
	slices.Reverse(segs)
	return strings.Join(segs, "")
}

func keyText(k value.Key) string {
	switch {
	case k.IsInt:
		return strconv.FormatInt(k.I, decimalBase)
	case word(k.S):
		return k.S
	}
	return string(diag.AppendJSONString(nil, k.S))
}

// word is an identifier of SPEC §2.4, `_` alone excepted.
func word(s string) bool {
	if s == "" || s == underscore {
		return false
	}
	for i, c := range s {
		letter := c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// mapKey is the key a map key value is written with in a path.
func mapKey(v value.Value) value.Key {
	switch k := v.(type) {
	case *value.Int:
		return value.Key{I: k.V, IsInt: true}
	case *value.Ref:
		return k.Key
	}
	return value.Key{S: v.CanonText()}
}
