package lock

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// coll is a locked collection: a stable table, a @codes enum or a @stable field of a table.
type coll struct {
	kind Kind
	name string // the written name: a field's is `<table>.<field>`
}

// Sources are the current facts of one package (LOCK.md §3), each located in the sources.
type Sources struct {
	facts *File
	colls map[coll]bool
	skip  map[coll]bool
}

// NewSources is the empty set of current facts of package pkg.
func NewSources(pkg string) *Sources {
	return &Sources{facts: New(pkg), colls: map[coll]bool{}, skip: map[coll]bool{}}
}

// Skip marks an existing collection whose facts are unknown, poisoned or broken: the lock is
// not compared with it, nor with a skipped table's @stable fields.
func (s *Sources) Skip(kind Kind, name string) {
	c := coll{kind: kind, name: name}
	s.colls[c], s.skip[c] = true, true
}

// skipped reports a collection Skip names, or a @stable field of a skipped table.
func (s *Sources) skipped(c coll) bool {
	if s.skip[c] {
		return true
	}
	i := strings.LastIndex(c.name, nameSep)
	return c.kind == KindField && i >= 0 && s.skip[coll{kind: KindTable, name: c.name[:i]}]
}

// AddTable adds a stable table's facts (a table fact per entry, a field fact per @stable value)
// under its qualified let name, reusing prev, an earlier AddTable's Order or nil, and returns
// its own Order, nil when a fact was refused.
func (s *Sources) AddTable(name string, t *value.Table, prev *Order) (*Order, error) {
	tt, ok := t.T.Base().(*types.TableType)
	if !ok || !tt.Stable {
		return nil, fmt.Errorf(fmtNotStable, ErrNotLocked, name)
	}
	s.colls[coll{kind: KindTable, name: name}] = true
	stable := stableFields(tt.Elem)
	for _, f := range stable {
		s.colls[coll{kind: KindField, name: name + nameSep + f.Name}] = true
	}
	batch := make([]Fact, 0, len(t.Entries)*(len(stable)+1))
	for _, e := range t.Entries {
		if e != nil && e.Ident != nil {
			batch = appendEntry(batch, name, e, stable)
		}
	}
	if prev.orders(s.facts.Package, batch) {
		s.facts.mergeRun(prev.run(batch))
		return &Order{pkg: prev.pkg, facts: batch, kept: prev.kept, retired: prev.retired}, nil
	}
	return s.merge(batch)
}

// appendEntry appends an entry's table fact, then a field fact per @stable field it holds a
// lockable value in.
func appendEntry(batch []Fact, name string, e *value.Record, stable []*types.Field) []Fact {
	key := e.Ident.Key.Text()
	batch = append(batch, Fact{Kind: KindTable, Name: name, Holder: key, Retired: e.Ident.Retired, Span: verify.SiteOf(e).Span})
	for _, f := range stable {
		if f.Index >= len(e.Fields) || e.Fields[f.Index] == nil {
			continue
		}
		if v, ok := lockValue(e.Fields[f.Index]); ok {
			batch = append(batch, Fact{Kind: KindField, Name: name, Field: f.Name, Value: v, Holder: key, Span: verify.SiteOf(e.Fields[f.Index]).Span})
		}
	}
	return batch
}

// stableFields is the @stable fields of a table's element record.
func stableFields(elem types.Type) []*types.Field {
	var out []*types.Field
	if r, ok := elem.Base().(*types.RecordType); ok {
		for _, f := range r.Fields {
			if f.Stable {
				out = append(out, f)
			}
		}
	}
	return out
}

// lockValue is a @stable value as the lock writes it: an integer or a string (LOCK.md §2.2).
func lockValue(v value.Value) (Value, bool) {
	switch x := v.(type) {
	case *value.Int:
		return Value{Int: x.V}, true
	case *value.Str:
		return Value{IsString: true, Str: x.V}, true
	}
	return Value{}, false
}

// AddEnum adds a @codes enum's facts, one per member with its code and retirement; an enum
// without @codes locks nothing.
func (s *Sources) AddEnum(enum check.Object) error {
	e, ok := enum.Type().(*types.EnumType)
	if !ok || e.Codes == nil {
		return nil
	}
	name := enum.Pkg() + nameSep + e.Name
	s.colls[coll{kind: KindEnum, name: name}] = true
	spans := memberSpans(enum.File(), e)
	batch := make([]Fact, len(e.Members))
	for i, m := range e.Members {
		batch[i] = Fact{Kind: KindEnum, Name: name, Value: Value{Int: m.Code}, Holder: m.Name, Retired: m.Retired, Span: spans[i]}
	}
	_, err := s.merge(batch)
	return err
}

// memberSpans locates each member of an enum declared in file, by name.
func memberSpans(file *syntax.File, e *types.EnumType) []source.Span {
	out := make([]source.Span, len(e.Members))
	if e.Decl == nil || file == nil {
		return out
	}
	byName := map[string]source.Span{}
	for _, m := range e.Decl.Members {
		if m != nil && m.Name != nil {
			byName[m.Name.Name] = file.Span(m)
		}
	}
	for i, m := range e.Members {
		out[i] = byName[m.Name]
	}
	return out
}
