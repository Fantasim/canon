package verify

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Path is a canonical value path built one segment at a time, nil for none (API.md §6.5).
type Path struct {
	parent *Path
	seg    string
	form   segForm
}

// segForm is how a segment is written: the root's name, `.name` or `[key]` (API.md P8).
type segForm uint8

// Root is the path of a top-level value.
func Root(name string) *Path {
	return &Path{seg: name, form: segRoot}
}

// Field is the path of a field of the record or case at p.
func (p *Path) Field(name string) *Path {
	return p.add(name, segField)
}

// Index is the path of element i of the plain list at p.
func (p *Path) Index(i int) *Path {
	return p.add(strconv.Itoa(i), segKey)
}

// Key is the path of the keyed-list element or map entry of key k at p (API.md P8, P9).
func (p *Path) Key(k value.Key) *Path {
	return p.add(keyText(k), segKey)
}

// MapKey is the path of the map entry of key k at p, kt the key type declared there: a literal of a literal-union key type is a JSON string (API.md P9).
func (p *Path) MapKey(k value.Value, kt types.Type) *Path {
	if s, ok := k.(*value.Str); ok && isLiteral(s.V, kt) {
		return p.quoted(s.V)
	}
	return p.Key(KeyOf(k))
}

// quoted is the path of the map entry of the literal key s at p, written as a JSON string (API.md P9).
func (p *Path) quoted(s string) *Path {
	return p.add(string(diag.AppendJSONString(nil, s)), segKey)
}

// mapKeyType is the key type a map value is stored with; nil for a dependent map, keyed by refs.
func mapKeyType(m *value.Map) types.Type {
	if mt, ok := underOf(nil, m.T).(*types.MapType); ok {
		return mt.Key
	}
	return nil
}

// isLiteral reports s one of the literals of kt, a literal-union type under aliases and refinements (TYPES.md §4.1).
func isLiteral(s string, kt types.Type) bool {
	for kt != nil {
		u, ok := kt.Base().(*types.LitUnionType)
		if !ok {
			return false
		}
		if slices.Contains(u.Literals, s) {
			return true
		}
		kt = u.Of
	}
	return false
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
	var segs []*Path
	for q := p; q != nil; q = q.parent {
		segs = append(segs, q)
	}
	var sb strings.Builder
	for _, s := range slices.Backward(segs) {
		sb.WriteString(segOpen[s.form])
		sb.WriteString(s.seg)
		sb.WriteString(segClose[s.form])
	}
	return sb.String()
}

func (p *Path) add(seg string, form segForm) *Path {
	if p == nil {
		return nil
	}
	return &Path{parent: p, seg: seg, form: form}
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

// KeyOf is the key a map key value is written with in a path (API.md P9); MapKey also quotes
// the literals of a literal-union key type.
func KeyOf(v value.Value) value.Key {
	switch k := v.(type) {
	case *value.Int:
		return value.Key{I: k.V, IsInt: true}
	case *value.Ref:
		return k.Key
	}
	return value.Key{S: v.CanonText()}
}
