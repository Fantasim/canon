package lock_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// object is a fixture check.Object for an enum declaration.
type object struct {
	pkg  string
	typ  types.Type
	file *syntax.File
}

func (o *object) Kind() check.ObjKind { return check.ObjTypeName }
func (o *object) Name() string        { return "" }
func (o *object) Pkg() string         { return o.pkg }
func (o *object) Type() types.Type    { return o.typ }
func (o *object) Decl() syntax.Node   { return nil }
func (o *object) File() *syntax.File  { return o.file }

// sourcesOf builds the current facts of package pkg the way a build does, without the
// evaluator: each stable table literal becomes a value.Table whose entries hold the literal
// values of their @stable fields, and each @codes enum a types.EnumType.
func sourcesOf(t *testing.T, pkg string, files []*syntax.File) *lock.Sources {
	t.Helper()
	s := lock.NewSources(pkg)
	records := map[string]*types.RecordType{}
	for _, f := range files {
		for _, d := range f.Decls {
			if r, ok := d.(*syntax.RecordDecl); ok {
				records[r.Name.Name] = recordOf(pkg, r)
			}
		}
	}
	for _, f := range files {
		for _, d := range f.Decls {
			var err error
			switch d := d.(type) {
			case *syntax.LetDecl:
				err = addTable(s, pkg, f, d, records)
			case *syntax.EnumDecl:
				err = s.AddEnum(&object{pkg: pkg, typ: enumOf(pkg, d), file: f})
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return s
}

func recordOf(pkg string, d *syntax.RecordDecl) *types.RecordType {
	r := &types.RecordType{Pkg: pkg, Name: d.Name.Name}
	for _, item := range d.Body.Items {
		if fd, ok := item.(*syntax.FieldDecl); ok {
			r.Fields = append(r.Fields, &types.Field{Name: fd.Name.Name, Index: len(r.Fields), Type: types.StringType, Stable: annotated(fd.Annotations, "stable")})
		}
	}
	return r
}

func annotated(anns []*syntax.Annotation, name string) bool {
	for _, a := range anns {
		if a.Name.Name == name {
			return true
		}
	}
	return false
}

func retired(m *syntax.Modifiers) bool { return m != nil && m.Retired.Valid() }

// addTable adds a `let name: stable table T = { … }` of literal entries.
func addTable(s *lock.Sources, pkg string, f *syntax.File, d *syntax.LetDecl, records map[string]*types.RecordType) error {
	tt, ok := d.Type.(*syntax.TableType)
	lit, isLit := d.Value.(*syntax.BraceLit)
	if !ok || !tt.Stable.Valid() || !isLit {
		return nil
	}
	parts := tt.Name.Parts
	elem := records[parts[len(parts)-1].Name]
	table := &value.Table{T: &types.TableType{Elem: elem, Stable: true}}
	coll := &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: d.Name.Name, Elem: elem}
	for _, item := range lit.Items {
		e, ok := item.(*syntax.EntryItem)
		if !ok {
			continue
		}
		rec := &value.Record{
			T: elem, Fields: make([]value.Value, len(elem.Fields)),
			Ident: &value.Identity{Coll: coll, Key: value.Key{S: e.Key.Name}, Retired: retired(e.Mods)},
			P:     &value.Prov{Span: f.Span(e.Key).Cover(f.Span(e.Value))},
		}
		for _, fi := range e.Value.Items {
			if field, ok := fi.(*syntax.FieldItem); ok {
				setField(rec, elem, field, f.Span(field.Value))
			}
		}
		table.Entries = append(table.Entries, rec)
	}
	return s.AddTable(pkg+"."+d.Name.Name, table)
}

// setField stores an integer or plain string literal given to a field.
func setField(rec *value.Record, elem *types.RecordType, fi *syntax.FieldItem, at source.Span) {
	for i, f := range elem.Fields {
		if f.Name != fi.Name.Name {
			continue
		}
		p := &value.Prov{Span: at}
		switch v := fi.Value.(type) {
		case *syntax.IntLit:
			rec.Fields[i] = &value.Int{V: v.Value.Int64(), T: types.IntType, P: p}
		case *syntax.StringLit:
			rec.Fields[i] = &value.Str{V: v.Parts[0].Text, T: types.StringType, P: p}
		}
	}
}

func enumOf(pkg string, d *syntax.EnumDecl) *types.EnumType {
	e := &types.EnumType{Pkg: pkg, Name: d.Name.Name, Decl: d}
	if annotated(d.Annotations, "codes") {
		e.Codes = &types.UInt16Type
	}
	for i, m := range d.Members {
		code, ok := m.Value.(*syntax.IntLit)
		member := &types.Member{Name: m.Name.Name, Index: i, Retired: retired(m.Mods), HasCode: ok}
		if ok {
			member.Code = code.Value.Int64()
		}
		e.Members = append(e.Members, member)
	}
	return e
}
