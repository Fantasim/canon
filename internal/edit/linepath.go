package edit

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// eachLine calls do with each line of f amending root r, and the path it amends as an operation
// names it.
func (s *Snapshot) eachLine(f *syntax.File, r Path, do func(string, *syntax.Amendment)) {
	for _, b := range f.Amends {
		if b.Target == nil || b.Target.Name != r.Root {
			continue
		}
		for _, it := range b.Items {
			if len(it.Path) > 0 {
				do(s.linePathOf(f, r, it.Path), it)
			}
		}
	}
}

// linePathOf is the path an amend line of f on root r names, as an operation names it: a key
// that names a word (a quoted name, a qualified member) as that word, a position or an index from
// the end as the item it names in s (keyExprIs, API.md P9, EVALUATION.md 9.2).
func (s *Snapshot) linePathOf(f *syntax.File, r Path, segs []*syntax.AmendSegment) string {
	var b strings.Builder
	b.WriteString(r.String())
	for _, seg := range segs {
		b.WriteString(s.segText(f, b.String(), seg))
	}
	return b.String()
}

// segText is one amend segment after prefix, the path its earlier segments name.
func (s *Snapshot) segText(f *syntax.File, prefix string, seg *syntax.AmendSegment) string {
	switch {
	case seg.Name != nil:
		return string(fieldMark) + seg.Name.Name
	case seg.Position != nil && seg.Position.Value.IsInt64():
		if text, ok := s.itemAt(prefix, seg.Position.Value.Int64()); ok {
			return text
		}
		return string(bracketOpen) + positionMark + seg.Position.Value.String() + bracketClose
	}
	if name, ok := s.keyWord(seg.Key); ok {
		return string(bracketOpen) + name + bracketClose
	}
	if n, ok := syntax.Unparen(seg.Key).(*syntax.IntLit); ok {
		if text, ok := s.fromEnd(prefix, n); ok {
			return text
		}
	}
	if str, ok := constString(syntax.Unparen(seg.Key)); ok {
		return Path{Segs: []Seg{{Kind: SegKey, Key: quotedKey(str)}}}.String() // its escapes as a path writes them
	}
	sp := f.Span(seg.Key)
	return string(bracketOpen) + string(f.Src.Content[sp.Start:sp.End]) + bracketClose
}

// itemAt is the segment naming the item at position i of the collection at prefix in s.
func (s *Snapshot) itemAt(prefix string, i int64) (string, bool) {
	c, ok := s.valueOf(prefix)
	if !ok || i < 0 {
		return "", false
	}
	var seg Seg
	switch x := c.(type) {
	case *value.List:
		if i >= int64(len(x.Elems)) {
			return "", false
		}
		seg = intSeg(i)
	case *value.Map:
		if i >= int64(len(x.Keys)) {
			return "", false
		}
		kt, _, _ := mapTypes(x.T)
		seg = keySeg(x.Keys[i], kt)
	case *value.Table:
		if i >= int64(len(x.Entries)) || x.Entries[i].Ident == nil {
			return "", false
		}
		seg = entrySeg(x.Entries[i].Ident.Key)
	default:
		return "", false
	}
	return Path{Segs: []Seg{seg}}.String(), true
}

// fromEnd is the segment naming element k, counted from the end, of the plain list at prefix in
// s (fromEndIndex): a keyed list's negative key is a key.
func (s *Snapshot) fromEnd(prefix string, k *syntax.IntLit) (string, bool) {
	c, ok := s.valueOf(prefix)
	if !ok {
		return "", false
	}
	if i, plain := fromEndIndex(c, k); plain && i >= 0 {
		return string(bracketOpen) + strconv.FormatInt(i, decimalBase) + bracketClose, true
	}
	return "", false
}

// valueOf is the value at path in s.
func (s *Snapshot) valueOf(path string) (value.Value, bool) {
	p, err := Parse(path)
	if err != nil {
		return nil, false
	}
	res, err := s.open(p)
	return res.Target, err == nil && res.Target != nil
}

// keyWord is the word an amend key names, as keyMatches reads it: a bare or quoted name, or a
// qualified member or case; false for any other key.
func (s *Snapshot) keyWord(e syntax.Expr) (string, bool) {
	switch x := syntax.Unparen(e).(type) {
	case *syntax.IdentExpr:
		return x.Name, true
	case *syntax.SelectorExpr:
		return x.Name.Name, s.info.Selections[x] == nil
	}
	return "", false
}
