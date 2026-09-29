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
	batch []Fact // checked facts not yet merged into facts
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

// add checks a fact and queues it for flush, which merges the queue in one pass.
func (s *Sources) add(fact Fact) error {
	if err := s.facts.valid(fact); err != nil {
		return err
	}
	s.batch = append(s.batch, fact)
	return nil
}

// flush merges the queued facts into the set; the facts queued before an error stay added.
func (s *Sources) flush() {
	s.facts.mergeAll(s.batch)
	s.batch = nil
}

// AddTable adds a stable table's facts: a table fact per entry, live or retired, and a field
// fact per entry and @stable field of its element; name is the table's qualified let.
func (s *Sources) AddTable(name string, t *value.Table) error {
	tt, ok := t.T.Base().(*types.TableType)
	if !ok || !tt.Stable {
		return fmt.Errorf(fmtNotStable, ErrNotLocked, name)
	}
	defer s.flush()
	s.colls[coll{kind: KindTable, name: name}] = true
	stable := stableFields(tt.Elem)
	for _, f := range stable {
		s.colls[coll{kind: KindField, name: name + nameSep + f.Name}] = true
	}
	for _, e := range t.Entries {
		if e == nil || e.Ident == nil {
			continue
		}
		if err := s.addEntry(name, e, stable); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sources) addEntry(name string, e *value.Record, stable []*types.Field) error {
	key := e.Ident.Key.Text()
	fact := Fact{Kind: KindTable, Name: name, Holder: key, Retired: e.Ident.Retired, Span: verify.SiteOf(e).Span}
	if err := s.add(fact); err != nil {
		return err
	}
	for _, f := range stable {
		if f.Index >= len(e.Fields) || e.Fields[f.Index] == nil {
			continue
		}
		v, ok := lockValue(e.Fields[f.Index])
		if !ok {
			continue
		}
		fact := Fact{Kind: KindField, Name: name, Field: f.Name, Value: v, Holder: key, Span: verify.SiteOf(e.Fields[f.Index]).Span}
		if err := s.add(fact); err != nil {
			return err
		}
	}
	return nil
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
	defer s.flush()
	name := enum.Pkg() + nameSep + e.Name
	s.colls[coll{kind: KindEnum, name: name}] = true
	spans := memberSpans(enum.File(), e)
	for i, m := range e.Members {
		fact := Fact{Kind: KindEnum, Name: name, Value: Value{Int: m.Code}, Holder: m.Name, Retired: m.Retired, Span: spans[i]}
		if err := s.add(fact); err != nil {
			return err
		}
	}
	return nil
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
