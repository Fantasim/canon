package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// splice is a change of a file's text: bytes lo to hi become text; added are the offsets in text
// of the identifiers it inserts, dropped the file's offsets of those it removes (API.md E35).
type splice struct {
	lo, hi         int
	text           string
	added, dropped []int
}

// wireSplice is the change a renamed field's `@json` takes (API.md E34): its old wire name as
// the positional argument, when it has none, nor `path:`, `pairs:` or `inline`, and its record or
// case is on the wire; conversely, a positional wire name the new name's default gives removed.
func (s *Snapshot) wireSplice(f *syntax.File, fd *syntax.FieldDecl, field *types.Field, newName string) (splice, bool) {
	ann := annotationNamed(fd.Annotations, syntax.AnnJSON)
	if arg := positionalArg(ann); arg != nil {
		if stringText(arg.Value) != check.ConvertCase(newName, s.wireCase(f, fd)) {
			return splice{}, false
		}
		return dropArg(f, ann, arg), true
	}
	if blocksWireName(ann) || !s.onWire(f, fd) {
		return splice{}, false
	}
	name := canonQuote(field.Wire)
	if ann == nil {
		end := int(f.Span(fd).End)
		text := space + annotationMark + syntax.AnnJSON + callOpen + name + callClose
		return splice{lo: end, hi: end, text: text, added: []int{len(space + annotationMark)}}, true
	}
	at := int(f.Tokens[ann.Parens.Open].End)
	if len(ann.Args) > 0 {
		name += listSep
	}
	return splice{lo: at, hi: at, text: name}, true
}

// annotationNamed is the annotation of that name among anns, nil when there is none.
func annotationNamed(anns []*syntax.Annotation, name string) *syntax.Annotation {
	i := slices.IndexFunc(anns, func(a *syntax.Annotation) bool { return a.Name != nil && a.Name.Name == name })
	if i < 0 {
		return nil
	}
	return anns[i]
}

// positionalArg is a `@json`'s positional wire name, nil when it has none.
func positionalArg(ann *syntax.Annotation) *syntax.AnnotationArg {
	if ann == nil {
		return nil
	}
	i := slices.IndexFunc(ann.Args, func(a *syntax.AnnotationArg) bool {
		_, isStr := a.Value.(syntax.StrLit)
		return a.Name == nil && isStr
	})
	if i < 0 {
		return nil
	}
	return ann.Args[i]
}

// blocksWireName reports a `@json` whose `path:`, `pairs:` or `inline` excludes a wire name.
func blocksWireName(ann *syntax.Annotation) bool {
	if ann == nil {
		return false
	}
	return slices.ContainsFunc(ann.Args, func(a *syntax.AnnotationArg) bool {
		if a.Name != nil {
			return a.Name.Name == jsonArgPath || a.Name.Name == jsonArgPairs
		}
		q, isSym := a.Value.(*syntax.QualifiedName)
		return isSym && syntax.Qualified(q) == jsonArgInline
	})
}

// stringText is a constant string argument's text.
func stringText(v syntax.AnnValue) string {
	switch x := v.(type) {
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			b.WriteString(p.Text)
		}
		return b.String()
	case *syntax.RawStringLit:
		return x.Value
	}
	return ""
}

// dropArg removes arg from ann, and ann with it when nothing else is left in it.
func dropArg(f *syntax.File, ann *syntax.Annotation, arg *syntax.AnnotationArg) splice {
	if len(ann.Args) == 1 {
		return splice{lo: int(f.Tokens[ann.First()-1].End), hi: int(f.Span(ann).End), dropped: []int{int(f.Span(ann.Name).Start)}}
	}
	i := slices.Index(ann.Args, arg)
	if i+1 < len(ann.Args) {
		return splice{lo: int(f.Span(arg).Start), hi: int(f.Span(ann.Args[i+1]).Start)}
	}
	return splice{lo: int(f.Span(ann.Args[i-1]).End), hi: int(f.Span(arg).End)}
}

// wireCase is the `@json(case:)` a field's default wire name follows: its case's, else its
// variant's, for a case field; its record's for a record field (WIRE.md 5.5.2).
func (s *Snapshot) wireCase(f *syntax.File, fd *syntax.FieldDecl) string {
	chain := holders(f, fd)
	for _, n := range slices.Backward(chain) {
		var anns []*syntax.Annotation
		switch d := n.(type) {
		case *syntax.RecordDecl:
			anns = d.Annotations
		case *syntax.VariantCase:
			anns = d.Annotations
		case *syntax.VariantDecl:
			anns = d.Annotations
		}
		if c := caseArg(annotationNamed(anns, syntax.AnnJSON)); c != "" {
			return c
		}
	}
	return ""
}

// caseArg is the style a `@json` names with `case:`, "" for none.
func caseArg(ann *syntax.Annotation) string {
	if ann == nil {
		return ""
	}
	for _, a := range ann.Args {
		if q, isSym := a.Value.(*syntax.QualifiedName); isSym && a.Name != nil && a.Name.Name == syntax.ArgCase {
			return syntax.Qualified(q)
		}
	}
	return ""
}

// onWire reports a field whose record or case is reachable, never through a ref, from the
// expected type of a load or the declared type of a value an emit writes as data (API.md E34).
func (s *Snapshot) onWire(f *syntax.File, fd *syntax.FieldDecl) bool {
	chain := holders(f, fd)
	if len(chain) == 0 {
		return false
	}
	owner := s.info.Defs[itemName(chain[len(chain)-1])]
	if owner == nil || owner.Type() == nil {
		return false
	}
	return s.wireTypes()[typeKey(owner.Type())]
}

// typeKey is the record or case type t is, by which reachability is recorded.
func typeKey(t types.Type) types.Type {
	if ar, ok := t.Base().(*types.AppliedRecord); ok {
		return ar.Rec
	}
	return t.Base()
}

// wireTypes are the records and cases reachable from the types loads and emits read or write.
func (s *Snapshot) wireTypes() map[types.Type]bool {
	seen := map[types.Type]bool{}
	for _, p := range s.pkgs {
		for _, f := range p.Files {
			for _, t := range s.fileWireRoots(p, f) {
				reach(seen, t)
			}
		}
	}
	return seen
}

// fileWireRoots are the expected types of f's `load`, `load.dir` and `load.csv`, and the declared
// types of the values its emits write: `emit json`, and go, cpp or ts in a data mode.
func (s *Snapshot) fileWireRoots(p *check.Package, f *syntax.File) []types.Type {
	var out []types.Type
	syntax.Inspect(f, func(n syntax.Node) bool {
		if l, ok := n.(*syntax.LoadExpr); ok && (l.Method == nil || loadsData[l.Method.Name]) {
			out = append(out, s.info.Types[l])
		}
		return n != nil
	})
	for _, d := range f.Decls {
		if e, ok := d.(*syntax.EmitDecl); ok && emitsData(e) {
			out = append(out, s.emitted(p, e)...)
		}
	}
	return out
}

// emitsData reports an `emit json`, or a go, cpp or ts emit in mode embedded, data or types.
func emitsData(e *syntax.EmitDecl) bool {
	if e.Target == nil || e.Options == nil {
		return false
	}
	if e.Target.Name == syntax.AnnJSON {
		return true
	}
	mode := emitOption(e, check.OptMode)
	id, ok := mode.(*syntax.IdentExpr)
	return codeTargets[e.Target.Name] && ok && dataModes[id.Name]
}

// emitOption is the value of an emit's option, nil when it is not written.
func emitOption(e *syntax.EmitDecl, name string) syntax.Expr {
	for _, it := range e.Options.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Name.Name == name {
			return fi.Value
		}
	}
	return nil
}

// emitted are the declared types of the values an emit writes: those `values:` lists, else (or
// with an empty list) every public let of its package (WIRE.md 8.1, CODEGEN.md 2.1).
func (s *Snapshot) emitted(p *check.Package, e *syntax.EmitDecl) []types.Type {
	var out []types.Type
	if list, ok := emitOption(e, check.OptValues).(*syntax.ListLit); ok {
		for _, x := range list.Elems {
			if id, isName := x.(*syntax.IdentExpr); isName && s.info.Uses[id] != nil {
				out = append(out, s.info.Uses[id].Type())
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, obj := range p.Decls {
		if obj.Kind() == check.ObjLet && public(obj) {
			out = append(out, obj.Type())
		}
	}
	return out
}

// reach marks the records and cases t reaches through fields, case fields, list, map, optional
// and table elements and aliases, never a ref.
func reach(seen map[types.Type]bool, t types.Type) {
	if t == nil {
		return
	}
	switch b := t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType:
		k := typeKey(b)
		if !seen[k] {
			seen[k] = true
			for _, f := range fieldsOf(k) {
				reach(seen, f.Type)
			}
		}
	case *types.VariantType:
		for _, c := range b.Cases {
			reach(seen, c)
		}
	case *types.ListType:
		reach(seen, b.Elem)
	case *types.MapType:
		reach(seen, b.Value)
	case *types.OptionalType:
		reach(seen, b.Elem)
	case *types.TableType:
		reach(seen, b.Elem)
	}
}
