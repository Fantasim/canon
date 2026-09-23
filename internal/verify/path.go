package verify

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/value"
)

// Path is a canonical value path (API.md §6.5) built one segment at a time; nil is no path.
type Path struct {
	parent *Path
	seg    string
}

// Root is the path of a top-level value.
func Root(name string) *Path {
	return &Path{seg: name}
}

// Field is the path of a field of the record or case at p.
func (p *Path) Field(name string) *Path {
	return p.add(dot + name)
}

// Index is the path of element i of the plain list at p.
func (p *Path) Index(i int) *Path {
	return p.add(keyOpen + strconv.Itoa(i) + keyClose)
}

// Key is the path of the keyed-list element or map entry of key k at p (API.md P8, P9).
func (p *Path) Key(k value.Key) *Path {
	return p.add(keyOpen + keyText(k) + keyClose)
}

// Entry is the path of the table entry of key k at p: `.key`, or `["key"]` for a non-word.
func (p *Path) Entry(k value.Key) *Path {
	if !k.IsInt && isWord(k.S) {
		return p.Field(k.S)
	}
	return p.Key(k)
}

// String is the path's canonical text; "" for no path.
func (p *Path) String() string {
	if p == nil {
		return ""
	}
	var segs []string
	for q := p; q != nil; q = q.parent {
		segs = append(segs, q.seg)
	}
	var sb strings.Builder
	for _, s := range slices.Backward(segs) {
		sb.WriteString(s)
	}
	return sb.String()
}

func (p *Path) add(seg string) *Path {
	if p == nil {
		return nil
	}
	return &Path{parent: p, seg: seg}
}

// keyText writes an integer in decimal, a word bare and any other key as a JSON string (P9).
func keyText(k value.Key) string {
	if k.IsInt {
		return strconv.FormatInt(k.I, decimalBase)
	}
	if isWord(k.S) {
		return k.S
	}
	return string(diag.AppendJSONString(nil, k.S))
}

// isWord is a word of SPEC §2.4; "_" alone is not one.
func isWord(s string) bool {
	for i, c := range s {
		letter := c == '_' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return s != "" && s != underscore
}

// KeyOf is the key a map key value is written with in a path (API.md P9).
func KeyOf(v value.Value) value.Key {
	switch k := v.(type) {
	case *value.Int:
		return value.Key{I: k.V, IsInt: true}
	case *value.Ref:
		return k.Key
	}
	return value.Key{S: v.CanonText()}
}
