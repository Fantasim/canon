package progen_test

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// localDecl is a top-level `local` declaration; pinned when annotated or a stable table (LOCK.md §1).
type localDecl struct {
	decl   syntax.Decl
	name   *syntax.Ident
	doc    *syntax.DocComment
	pinned bool
	fresh  func(string) string
}

// localDecls are tg's top-level `local` declarations, in order (GRAMMAR.md §5.1).
func localDecls(tg target) []localDecl {
	var out []localDecl
	for _, d := range tg.file.Decls {
		if ld, ok := localOf(d); ok {
			out = append(out, ld)
		}
	}
	return out
}

// localOf is d as a localDecl, false when d is not a named `local` declaration.
func localOf(d syntax.Decl) (localDecl, bool) {
	ld := localDecl{decl: d, fresh: renameCamel}
	var mods *syntax.Modifiers
	switch v := d.(type) {
	case *syntax.ConstDecl:
		mods, ld.name, ld.doc, ld.pinned, ld.fresh = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0, renameConst
	case *syntax.LetDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0 || stableTable(v.Type)
	case *syntax.TypeDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0
	case *syntax.FnDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0
	case *syntax.RecordDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0
	case *syntax.EnumDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0
	case *syntax.VariantDecl:
		mods, ld.name, ld.doc, ld.pinned = v.Mods, v.Name, v.Doc, len(v.Annotations) > 0
	}
	return ld, mods != nil && mods.Local != syntax.NoTok && ld.name != nil
}

// stableTable tells a `stable table` type, whose keys canon.lock holds by the let's name.
func stableTable(t syntax.Type) bool {
	tt, ok := t.(*syntax.TableType)
	return ok && tt.Stable != syntax.NoTok
}

// renameConst and renameCamel keep GRAMMAR.md §9.2's naming convention so a rename never trips W1003.
func renameConst(name string) string { return name + "_2" }
func renameCamel(name string) string { return name + "Zz" }

// renameSites rename an unpinned `local` declaration of tg where every token spelling its name
// is the declaration or a name expression or type name resolving to it (never a field, member,
// key, argument name or a shadowing binding).
func renameSites(tg target) []metaSite {
	if !isSource(tg) {
		return nil
	}
	refs := referenceStarts(tg)
	var out []metaSite
	for _, ld := range localDecls(tg) {
		if ld.pinned {
			continue
		}
		if s, ok := renameSite(tg, ld, refs); ok {
			out = append(out, s)
		}
	}
	return out
}

// renameSite renames ld throughout tg, false when a token spells its name out of refs, a peer file
// spells it, or its fresh name is taken.
func renameSite(tg target, ld localDecl, refs map[int]bool) (metaSite, bool) {
	from, to := ld.name.Name, ld.fresh(ld.name.Name)
	if usedInPeers(tg, from) || usedInPeers(tg, to) || tokenExists(tg, to) {
		return metaSite{}, false
	}
	decl := tg.file.Span(ld.name).Start
	var edits []progen.Edit
	for _, tok := range tg.file.Tokens {
		if tok.Kind != syntax.TokIdent || string(tg.src[tok.Start:tok.End]) != from {
			continue
		}
		if !refs[int(tok.Start)] && tok.Start != decl {
			return metaSite{}, false
		}
		edits = append(edits, replace(int(tok.Start), int(tok.End), to))
	}
	return metaSite{edits: edits, desc: "rename " + from + " to " + to}, true
}

// referenceStarts are the offsets of tg's name expressions and one-part type names: the tokens
// that name a declaration by its plain name.
func referenceStarts(tg target) map[int]bool {
	refs := map[int]bool{}
	for _, e := range nodes[*syntax.IdentExpr](tg) {
		refs[int(tg.file.Span(e).Start)] = true
	}
	for _, q := range typeNames(tg) {
		if q != nil && len(q.Parts) == 1 {
			refs[int(tg.file.Span(q.Parts[0]).Start)] = true
		}
	}
	return refs
}

// typeNames are the names tg's named, ref and table types write.
func typeNames(tg target) []*syntax.QualifiedName {
	var out []*syntax.QualifiedName
	for _, t := range nodes[*syntax.NamedType](tg) {
		out = append(out, t.Name)
	}
	for _, t := range nodes[*syntax.RefType](tg) {
		out = append(out, t.Name)
	}
	for _, t := range nodes[*syntax.TableType](tg) {
		out = append(out, t.Name)
	}
	return out
}

// usedInPeers tells a whole-word occurrence of name in another file of tg's package.
func usedInPeers(tg target, name string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	for _, p := range peers(tg) {
		if p.path != tg.path && re.Match(p.src) {
			return true
		}
	}
	return false
}

// tokenExists tells an IDENT token spelled name anywhere in tg (never a string or a comment,
// which the lexer never tags TokIdent).
func tokenExists(tg target, name string) bool {
	for _, tok := range tg.file.Tokens {
		if tok.Kind == syntax.TokIdent && string(tg.src[tok.Start:tok.End]) == name {
			return true
		}
	}
	return false
}
