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
	return p.add(k.PathText(), segKey)
}

// MapKey is the path of the map entry of key k at p, kt the key type declared there: a literal of a literal-union key type is a JSON string (API.md P9).
func (p *Path) MapKey(k value.Value, kt types.Type) *Path {
	return p.add(value.PathKey(k, kt), segKey)
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

// Entry is the path of the table entry of key k at p: `.key`, or `["key"]` for a non-word.
func (p *Path) Entry(k value.Key) *Path {
	if !k.IsInt && value.IsWord(k.S) {
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
