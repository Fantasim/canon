package ir

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// discRoute walks app's discriminant path, the argument's then the match's wire path, down fields: through fields of records held by value, none optional, ending at the discriminant or at the first ref. It returns the fields read, the wire segments left after a ref, and whether the walk stopped at one; nil fields for an argument no loader reads (a record parameter's, a dependent map binder's, TYPES.md §11.1, §11.2) or a path it cannot follow.
func discRoute(fields []*Field, app TypeRef) ([]*Field, []string, bool) {
	d, ok := app.Named.(*Dependent)
	if !ok || d.Disc == nil || d.DiscParam < 0 || d.DiscParam >= len(app.Args) || app.Args[d.DiscParam] == nil {
		return nil, nil, false
	}
	src := app.Args[d.DiscParam]
	if src.From != types.ArgField {
		return nil, nil, false
	}
	segs := slices.Concat(src.WirePath, d.DiscPath)
	var out []*Field
	for len(segs) > 0 {
		f := fieldAtWire(fields, segs)
		if f == nil || f.Optional {
			return nil, nil, false
		}
		segs, out = segs[len(f.WirePath):], append(out, f)
		if f.Type.Kind == types.Ref {
			return out, segs, true
		}
		fields = nil
		if rec, ok := f.Type.Named.(*Record); ok && f.Type.Kind == types.Record {
			fields = rec.Fields
		}
	}
	return out, nil, false
}

// DiscRefFields are the fields a loader reads from fields to the ref that decides app's discriminant (CODEGEN.md §5.6, DECISIONS 350): read as DiscFields reads them, ending at that ref field, whose key looks the field's branch up in its build-time table (Field.Branches); nil when the path crosses no ref, meets an optional before it, or ends at it.
func DiscRefFields(fields []*Field, app TypeRef) []*Field {
	path, rest, crossed := discRoute(fields, app)
	if !crossed || len(rest) == 0 {
		return nil
	}
	return path
}

// refDecided reports a field f of fields whose dependent value, itself or its list's elements, a loader or a literal decides through a ref by f's build-time table (DECISIONS 350).
func refDecided(fields []*Field, f *Field) bool {
	app := HeldApp(f.Type)
	return app != nil && f.Branches != nil && DiscRefFields(fields, *app) != nil
}

// branch is the index of the branch key selects in t, and whether t holds one: a key absent from the table is refused as a Never arm is (CODEGEN.md §5.6, DECISIONS 350).
func (t *BranchTable) branch(key value.Key) (int, bool) {
	i, ok := slices.BinarySearchFunc(t.Keys, key, compareKeys)
	if !ok {
		return NoBranch, false
	}
	return t.Branches[i], true
}

// compareKeys orders keys as a BranchTable holds them: integers by value, strings by bytes.
func compareKeys(a, b value.Key) int {
	if a.IsInt || b.IsInt {
		return cmp.Compare(a.I, b.I)
	}
	return cmp.Compare(a.S, b.S)
}

// RefBranch is the branch a record value r selects for its field f, whose discriminant path crosses a ref (DiscRefFields): the key r holds down the path, looked up in f's table; false when r holds no such key or the table no branch for it, which a checked value never meets (a present value has a branch, TYPES.md §11.1).
func RefBranch(fields []*Field, f *Field, r *value.Record) (int, bool) {
	app := HeldApp(f.Type)
	if app == nil || f.Branches == nil {
		return NoBranch, false
	}
	path := DiscRefFields(fields, *app)
	v := value.Value(r)
	for _, pf := range path {
		v = recordField(v, pf.Name)
	}
	ref, ok := v.(*value.Ref)
	if !ok {
		return NoBranch, false
	}
	return f.Branches.branch(ref.Key)
}

// recordField is the value of the field named name of the record value v, nil for none.
func recordField(v value.Value, name string) value.Value {
	r, ok := v.(*value.Record)
	if !ok {
		return nil
	}
	i := slices.IndexFunc(ownFields(r.T), func(f *types.Field) bool { return f.Name == name })
	if i < 0 || i >= len(r.Fields) {
		return nil
	}
	return r.Fields[i]
}

// branchTables fills the table of every dependent field of the types stage E reached whose discriminant path crosses a ref, each ref on it into a let that is not @reload (CODEGEN.md §5.6, DECISIONS 350): the ref's targets are known at build time. A field whose path crosses another ref (a field's collection, a define table, a @reload value: a reload may add keys) keeps none, and a generator refuses it (E8019 DependentType).
func (s *stage) branchTables() {
	for _, t := range s.named { //canon:unordered each field's table is computed from its own path and values alone
		for _, class := range typeClasses(t) {
			fields, _ := classBody(class)
			for _, f := range fields {
				s.branchTable(fields, f)
			}
		}
	}
}

// typeClasses are a record itself, or a variant's cases; nothing for another type.
func typeClasses(t Type) []any {
	switch x := t.(type) {
	case *Record:
		return []any{x}
	case *Variant:
		return classAndCases(x)
	}
	return nil
}

// branchTable fills f.Branches when f's dependent value is decided through a ref whose path stage E can follow at build time (refPath), from the build's values.
func (s *stage) branchTable(fields []*Field, f *Field) {
	app := HeldApp(f.Type)
	if app == nil || f.Pairs != nil || f.Branches != nil {
		return
	}
	path, rest, crossed := discRoute(fields, *app)
	if !crossed || len(rest) == 0 {
		return
	}
	d, _ := app.Named.(*Dependent)
	ref := path[len(path)-1].Type.Ref
	if !s.refPath(ref, rest, d) {
		return
	}
	entries, _ := s.refEntries(ref) // a value the host cannot give follows its own finding (Host): an empty table, never one more
	bt := &BranchTable{}
	for _, e := range entries {
		if br, ok := s.entryBranch(e, ref, rest, d); ok {
			bt.Keys, bt.Branches = append(bt.Keys, e.Ident.Key), append(bt.Branches, br)
		}
	}
	f.Branches = bt
}

// refPath reports a path rest, read from a target of ref, that ends at a field of d's discriminant type and crosses only records held by value and refs into lets that are not @reload, so that every value on it is known at build time.
func (s *stage) refPath(ref *RefTarget, rest []string, d *Dependent) bool {
	fields, ok := s.targetFields(ref)
	for ok && len(rest) > 0 {
		f := fieldAtWire(fields, rest)
		if f == nil {
			return false
		}
		if rest = rest[len(f.WirePath):]; len(rest) == 0 {
			return f.Type.Kind == d.Disc.Kind && f.Type.Named == d.Disc.Named
		}
		fields, ok = s.stepFields(f)
	}
	return false
}

// stepFields are the fields a path continues into after f: a record's held by value, or the target's of a ref stage E reads at build time (targetFields).
func (s *stage) stepFields(f *Field) ([]*Field, bool) {
	if f.Type.Kind == types.Ref {
		return s.targetFields(f.Type.Ref)
	}
	rec, ok := f.Type.Named.(*Record)
	if !ok || f.Type.Kind != types.Record {
		return nil, false
	}
	return rec.Fields, true
}

// targetFields are the fields of the record ref targets when it is a let that is not @reload (DECISIONS 350).
func (s *stage) targetFields(ref *RefTarget) ([]*Field, bool) {
	if !s.buildTimeTarget(ref) {
		return nil, false
	}
	rec, ok := ref.Elem.(*Record)
	if !ok {
		return nil, false
	}
	return rec.Fields, true
}

// buildTimeTarget reports a ref into a let that is not @reload (DECISIONS 350).
func (s *stage) buildTimeTarget(ref *RefTarget) bool {
	return ref != nil && ref.Coll == types.CollLet && ref.Value != "" && !s.letReload(ref.Pkg, ref.Value)
}

// letReload reports a let name of package pkg declared @reload.
func (s *stage) letReload(pkg, name string) bool {
	u := s.units[pkg]
	if u == nil {
		return false
	}
	for _, obj := range u.cp.Decls {
		if d, ok := obj.Decl().(*syntax.LetDecl); ok && obj.Name() == name {
			return annotation(d.Annotations, syntax.AnnReload) != nil
		}
	}
	return false
}

// refEntries are the entries of the let ref targets, a table's or a keyed list's, sorted by key (compareKeys); false when its value is not known (a finding already says why).
func (s *stage) refEntries(ref *RefTarget) ([]*value.Record, bool) {
	key := defineKey{ref.Pkg, ref.Value}
	if entries, ok := s.targets[key]; ok {
		return entries, entries != nil
	}
	s.targets[key] = nil
	v, ok := s.in.Host.Value(s.ctx, ref.Pkg, ref.Value)
	if !ok {
		return nil, false
	}
	entries := []*value.Record{} // empty, not nil: a let without entries is known
	switch x := v.(type) {
	case *value.Table:
		entries = append(entries, x.Entries...)
	case *value.List:
		for _, e := range x.Elems {
			if r, ok := e.(*value.Record); ok && r.Ident != nil {
				entries = append(entries, r)
			}
		}
	default:
		return nil, false
	}
	slices.SortStableFunc(entries, func(a, b *value.Record) int { return compareKeys(a.Ident.Key, b.Ident.Key) })
	s.targets[key] = entries
	return entries, true
}

// entryBranch is the branch the target entry e of ref selects down rest (refPath holds): the index in d's Branches of the arm its discriminant takes; false for a none met on the path or a Never arm (DECISIONS 307, 350).
func (s *stage) entryBranch(e *value.Record, ref *RefTarget, rest []string, d *Dependent) (int, bool) {
	fields, _ := s.targetFields(ref)
	cur := value.Value(e)
	for len(rest) > 0 {
		f := fieldAtWire(fields, rest)
		rest, cur = rest[len(f.WirePath):], recordField(cur, f.Name)
		if x, ok := cur.(*value.Ref); ok && len(rest) > 0 {
			cur = s.entry(f.Type.Ref, x.Key)
		}
		if cur == nil || len(rest) == 0 {
			break
		}
		fields, _ = s.stepFields(f)
	}
	return discBranch(cur, d)
}

// entry is the target of ref named by key, nil for none.
func (s *stage) entry(ref *RefTarget, key value.Key) value.Value {
	entries, ok := s.refEntries(ref)
	if !ok {
		return nil
	}
	i, found := slices.BinarySearchFunc(entries, key, func(e *value.Record, k value.Key) int { return compareKeys(e.Ident.Key, k) })
	if !found {
		return nil
	}
	return entries[i]
}

// discBranch is the branch of d a discriminant value v selects, false for a Never arm or a value that is no discriminant.
func discBranch(v value.Value, d *Dependent) (int, bool) {
	m := -1
	switch x := v.(type) {
	case *value.Member:
		m = x.Index
	case *value.Bool:
		if m = 0; x.V {
			m = 1
		}
	}
	if m < 0 || m >= len(d.ByMember) || d.ByMember[m] == NoBranch {
		return NoBranch, false
	}
	return d.ByMember[m], true
}
