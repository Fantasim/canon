package progen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on literals, optionals, refs, matches, assets and dependent types.
func valuesOperators() []operator {
	return []operator{
		op(diag.E3202.Def().Code, "TYPES.md §7.3 (Float32 overflow)", appendSite("local const ZZ_BIG = 1e38\nlocal let zzF: Float32 = ", "ZZ_BIG * 10.0", "")),
		op(diag.E3206.Def().Code, "TYPES.md §7.4 (where predicate false)", appendSite("local let zzW: Int where it % 5 == 0 = ", "7", "")),
		op(diag.E3301.Def().Code, "TYPES.md §5.2 (unknown field in a literal)", afterRecordItem(", ", "zzExtra", ": 1")),
		op(diag.W3301.Def().Code, "TYPES.md §16 (deprecated field given)", deprecatedFieldUse),
		op(diag.E3302.Def().Code, "TYPES.md §5.2 (missing required field)", missingRequired),
		op(diag.E3303.Def().Code, "TYPES.md §5.2 (spread of a table)", spreadTable),
		op(diag.E3304.Def().Code, "TYPES.md §5.2 (identifier map key)", fnStatementFocus("let zzMap: {String: Int} = { ", "a", ": 1 }")),
		op(diag.E3305.Def().Code, "TYPES.md §5.2 (undecidable brace literal)", fnStatementFocus("let zzBrace = ", "{ a: 1 }", "")),
		op(diag.E3306.Def().Code, "TYPES.md §12.3 (function type as a field)", fnField),
		op(diag.E3307.Def().Code, "TYPES.md §12.7 (assign to a field)", assignField),
		op(diag.E3308.Def().Code, "TYPES.md §6.4 (incompatible branches)", fnStatementFocus("let zzIf = if true { 1 } else { ", `"a"`, " }")),
		op(diag.E3309.Def().Code, "TYPES.md §7.5 (refs into two collections)", refsIntoTwoTables),
		op(diag.E3310.Def().Code, "TYPES.md §7.5 (ordering Bools)", fnStatementFocus("let zzLt = ", "true < false", "")),
		op(diag.E3311.Def().Code, "TYPES.md §5.3 (Int where Float)", fnStatementFocus("let zzF: Float = ", "[1].len()", "")),
		op(diag.E3312.Def().Code, "TYPES.md §14 (literal gives an input)", giveInput),
		op(diag.E3313.Def().Code, "TYPES.md §14 (input read at build time)", readInput),
		op(diag.E3314.Def().Code, "STDLIB.md §4.3 (sum of an unknown list)", fnStatementFocus("let zzSum = ", "[].sum()", "")),
		op(diag.E3316.Def().Code, "WIRE.md §4.1 (unit on a non-Duration)", fieldOfType("Int", " ", "@json(unit: s)")),
		op(diag.E3318.Def().Code, "WIRE.md §4.2 (two inline fields)", twoInlineFields),
		op(diag.E3320.Def().Code, "TYPES.md §5.2 (entry in a record literal)", afterRecordItem(", ", "zz", " { }")),
		op(diag.E3321.Def().Code, "TYPES.md §5.2 (field given twice)", fieldGivenTwice),
		op(diag.E3323.Def().Code, "TYPES.md §5.2 (spread after an item)", spreadNotFirst),
		op(diag.E3322.Def().Code, "TYPES.md §5.2 (map key twice)", fnStatementFocus(`let zzM: {String: Int} = { "a": 1, `, `"a"`, ": 2 }")),
		op(diag.E3401.Def().Code, "TYPES.md §2 (optional of an optional)", doubleOptional),
		op(diag.W3401.Def().Code, "TYPES.md §6.5 (! on a value never none)", forceParameter),
		op(diag.E3402.Def().Code, "TYPES.md §6.5 (member of an optional)", fnStatementFocus("let zzOpt: [Int]? = none\n  let zzLen = ", "zzOpt", ".len()")),
		op(diag.E3403.Def().Code, "TYPES.md §6.5 (optional for a present)", fnStatementFocus("let zzOpt: Int? = none\n  let zzInt: Int = ", "zzOpt", "")),
		op(diag.E3501.Def().Code, "TYPES.md §10.3, §4.1 (unknown key of a keyed list)", appendSite(
			"/// Row.\nlocal record ZzRow {\n  /// Key.\n  k: String\n}\n\nlocal let zzRows: [ZzRow] keyed by k = [ZzRow { k: \"a\" }]\n\nlocal let zzPick: ref zzRows = ",
			"zzmissing", "")),
		op(diag.E3502.Def().Code, "TYPES.md §10.3 (live entry names a retired one)", refToRetired),
		op(diag.E3504.Def().Code, "TYPES.md §10.2 (ref to a const)", refToConst),
		op(diag.E3505.Def().Code, "TYPES.md §10.2 (per-instance ref outside any instance)", appendSite(
			"/// Node.\nlocal record ZzNode {\n  /// Key.\n  name: String\n}\n\n/// Link.\nlocal record ZzLink {\n  /// To.\n  to: ref ZzNode\n}\n\n/// Tree.\nlocal record ZzTree {\n  /// Nodes.\n  nodes: [ZzNode] keyed by name\n  /// Links.\n  links: [ZzLink]\n}\n\nlocal let zzOrphan: ZzLink = { to: ",
			`"x"`, " }")),
		op(diag.E3506.Def().Code, "TYPES.md §8.1 (retired member in a value)", retiredMemberUse),
		op(diag.E3601.Def().Code, "TYPES.md §12.6 (match not covering)", dropArm),
		op(diag.W3601.Def().Code, "TYPES.md §12.6 (unreachable _)", addArm(func(string) string { return "_" })),
		op(diag.E3602.Def().Code, "TYPES.md §12.6 (pattern already covered)", repeatPattern),
		op(diag.E3603.Def().Code, "TYPES.md §12.6 (pattern not a member)", addArm(func(string) string { return "zzNope" })),
		op(diag.E3604.Def().Code, "TYPES.md §12.6 (match on an Int)", fnStatementFocus("let zzM = match ", "1", " { _ => 1 }")),
		op(diag.E3605.Def().Code, "TYPES.md §8.3 (is on a non-variant)", fnStatementFocus("let zzIs = ", "1", " is zz")),
		op(diag.E3503.Def().Code, "TYPES.md §6.2 (a record that is no entry, as a ref)", refOfNonEntry),
		op(diag.E3701.Def().Code, "TYPES.md §13.4 (asset file missing)", assetHolder(`"zz_missing.dds"`)),
		op(diag.E3702.Def().Code, "TYPES.md §13.4 (asset extension)", assetHolder(`"Itm_ActivityPoints_50.png"`)),
		op(diag.E3703.Def().Code, "TYPES.md §13.4 (asset path not clean)", assetHolder(`"../Item/Itm_ActivityPoints_50.dds"`)),
		op(diag.E3704.Def().Code, "TYPES.md §13.4 (asset root not a load path)", appendSite(`local type ZzAsset = asset(`, `"/Icon/Item"`, ", ext: [dds])")),
		op(diag.E3801.Def().Code, "TYPES.md §11.6, §13.5 (required field whose computed type is Never)", requiredNever),
		op(diag.E3802.Def().Code, "TYPES.md §11.6 (value not fitting its computed type)", computedMismatch),
		op(diag.E3803.Def().Code, "TYPES.md §11.2 (type function on an Int)", appendSite("local record ZzEv {\n  n: Int\n}\n\nlocal type ZzP(e: ZzEv) = match ", "e.n", " {\n  _ => Int\n}")),
		op(diag.E3804.Def().Code, "TYPES.md §11.4 (method of a dependent value)", dependent("  check ", "p.len() > 0", ` else "empty"`)),
		op(diag.E3805.Def().Code, "TYPES.md §11.1 (field type uses a later field)", laterField),
		op(diag.E3806.Def().Code, "TYPES.md §11.1 (type function arity)", dependentArity),
	}
}

// recordLiterals are the brace literals of let values that name fields.
func recordLiterals(tg target) []*syntax.BraceLit {
	fields := fieldDecls(tg)
	var out []*syntax.BraceLit
	for _, b := range valueLiterals(tg) {
		if len(b.Items) > 0 && allFields(b, fields) {
			out = append(out, b)
		}
	}
	return out
}

// allFields tells a brace literal whose every item is a field the package declares: never a map
// literal, whose identifier keys are enum members (E2102 for a new one, not E3301).
func allFields(b *syntax.BraceLit, fields map[string][]*syntax.FieldDecl) bool {
	for _, it := range b.Items {
		f, ok := it.(*syntax.FieldItem)
		if !ok || len(fields[f.Name.Name]) == 0 {
			return false
		}
	}
	return true
}

// afterRecordItem inserts before+focus+after after the first item of every record literal.
func afterRecordItem(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, b := range recordLiterals(tg) {
			_, e := span(tg, b.Items[0])
			out = append(out, seq(1, insert(e, before), insert(e, focus), insert(e, after)))
		}
		return out
	}
}

func fieldGivenTwice(tg target) []progen.Site {
	var out []progen.Site
	for _, b := range recordLiterals(tg) {
		f, ok := b.Items[0].(*syntax.FieldItem)
		if !ok {
			continue
		}
		_, e := span(tg, f)
		out = append(out, keeping(tg, seq(1, insert(e, ", "), insert(e, f.Name.Name), insert(e, text(tg, f)[len(f.Name.Name):])), f.Name))
	}
	return out
}

// fieldDecls maps each field name of tg's package to its declarations.
func fieldDecls(tg target) map[string][]*syntax.FieldDecl {
	out := map[string][]*syntax.FieldDecl{}
	for _, p := range peers(tg) {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			out[f.Name.Name] = append(out[f.Name.Name], f)
		}
	}
	return out
}

// required tells a field name every declaration of which has no default and a non-optional type.
func required(decls []*syntax.FieldDecl) bool {
	for _, f := range decls {
		if _, opt := f.Type.(*syntax.OptionalType); opt || f.Default != nil || f.Input.Valid() {
			return false
		}
	}
	return len(decls) > 0
}

// dropItem is the edit that removes one item of a brace literal with its separator.
func dropItem(tg target, b *syntax.BraceLit, i int) progen.Edit {
	s, e := span(tg, b.Items[i])
	if startsLine(tg, b.Items[i]) && endsLine(tg, b.Items[i]) {
		return replace(lineStart(tg, s), lineEnd(tg, e)+1, "")
	}
	if i > 0 {
		_, pe := span(tg, b.Items[i-1])
		return replace(pe, e, "")
	}
	if len(b.Items) > 1 {
		ns, _ := span(tg, b.Items[1])
		return replace(s, ns, "")
	}
	return replace(s, e, "")
}

func missingRequired(tg target) []progen.Site {
	decls := fieldDecls(tg)
	var out []progen.Site
	for _, b := range recordLiterals(tg) {
		for i, it := range b.Items {
			if f, ok := it.(*syntax.FieldItem); ok && required(decls[f.Name.Name]) {
				s, _ := span(tg, b)
				out = append(out, seq(0, mark(tg, s, s+1), dropItem(tg, b, i)))
			}
		}
	}
	return out
}

// tableLets are the names of tg's package's lets whose type is a table.
func tableLets(tg target) []string {
	var out []string
	for _, p := range peers(tg) {
		for _, d := range nodes[*syntax.LetDecl](p) {
			if _, ok := d.Type.(*syntax.TableType); ok {
				out = append(out, d.Name.Name)
			}
		}
	}
	return out
}

func spreadTable(tg target) []progen.Site {
	var out []progen.Site
	for _, name := range tableLets(tg) {
		for _, b := range recordLiterals(tg) {
			s, _ := span(tg, b.Items[0])
			out = append(out, seq(1, insert(s, "..."), insert(s, name), insert(s, ", ")))
		}
	}
	return out
}

// recordField adds a documented field before+focus+after as the last item of every record.
func recordField(before, focus, after string) func(target) []progen.Site {
	return recordFieldOf(func(target) func(*syntax.RecordDecl) bool {
		return func(*syntax.RecordDecl) bool { return true }
	}, before, focus, after)
}

// fnField adds a function-typed field to a record that is no pairs element, whose count it would break (WIRE.md §4.1).
func fnField(tg target) []progen.Site {
	return recordFieldOf(func(tg target) func(*syntax.RecordDecl) bool {
		elems := pairsElements(tg)
		return func(r *syntax.RecordDecl) bool { return !elems[r.Name.Name] }
	}, "zzFn: ", "(fn(Int) -> Int)?", " = none")(tg)
}

// pairsElements are the names of the records any field of the project uses as a `@json(pairs:)` element.
func pairsElements(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range *tg.all {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			l, isList := f.Type.(*syntax.ListType)
			n, isNamed := elemNamed(l, isList)
			if isNamed && pairsForm(f.Annotations) {
				out[n.Name.Parts[len(n.Name.Parts)-1].Name] = true
			}
		}
	}
	return out
}

// elemNamed is the element of a list type when it is a named type.
func elemNamed(l *syntax.ListType, isList bool) (*syntax.NamedType, bool) {
	if !isList {
		return nil, false
	}
	n, ok := l.Elem.(*syntax.NamedType)
	return n, ok
}

// pairsForm reports a `@json(…)` with a `pairs:` argument among as.
func pairsForm(as []*syntax.Annotation) bool {
	for _, a := range as {
		if a.Name == nil || a.Name.Name != annotJSON {
			continue
		}
		for _, arg := range a.Args {
			if arg.Name != nil && arg.Name.Name == argPairs {
				return true
			}
		}
	}
	return false
}

// recordFieldOf adds before+focus+after as the last field of every record keep accepts.
func recordFieldOf(keepOf func(target) func(*syntax.RecordDecl) bool, before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		keep := keepOf(tg)
		return sitesOf(tg, func(r *syntax.RecordDecl) bool { return r.Body != nil && len(r.Body.Items) > 0 && keep(r) }, func(r *syntax.RecordDecl) progen.Site {
			_, e := span(tg, r.Body.Items[len(r.Body.Items)-1])
			ind := indent(tg, e)
			return seq(1, insert(e, "\n"+ind+"/// Added.\n"+ind+before), insert(e, focus), insert(e, after))
		})
	}
}

// assignField assigns to a field of a copy of self first in every method.
func assignField(tg target) []progen.Site {
	var out []progen.Site
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		if r.Body == nil || len(r.Body.Items) == 0 {
			continue
		}
		field, ok := r.Body.Items[0].(*syntax.FieldDecl)
		if !ok {
			continue
		}
		for _, it := range r.Body.Items {
			f, ok := it.(*syntax.FnDecl)
			if !ok || !f.Self.Valid() || f.Body == nil || len(f.Body.Stmts) == 0 || !startsLine(tg, f.Body.Stmts[0]) {
				continue
			}
			s, _ := span(tg, f.Body.Stmts[0])
			ind := indent(tg, s)
			out = append(out, seq(1, insert(s, "var zzCopy = self\n"+ind+"zzCopy."), insert(s, field.Name.Name),
				insert(s, " = zzCopy."+field.Name.Name+"\n"+ind)))
		}
	}
	return out
}

// refsIntoTwoTables adds a second table of a table's element type and compares refs into both;
// only for element types no ref names by type, which a second collection would make ambiguous.
func refsIntoTwoTables(tg target) []progen.Site {
	refs := referenced(tg)
	var out []progen.Site
	for _, d := range nodes[*syntax.LetDecl](tg) {
		t, ok := d.Type.(*syntax.TableType)
		if !ok || refs[text(tg, t.Name)] {
			continue
		}
		elem := text(tg, t.Name)
		before := "local let zzOther: table " + elem + " = {}\n\nlocal fn zzCmp(a: ref " + d.Name.Name + ", b: ref zzOther) -> Bool {\n  return "
		out = append(out, appendDecl(tg, before, "a == b", "\n}"))
	}
	return out
}

// inputHolders maps each record with input fields to the name of its first input field.
func inputHolders(tg target) map[string]string {
	out := map[string]string{}
	for _, p := range peers(tg) {
		for _, r := range nodes[*syntax.RecordDecl](p) {
			if name := firstInput(r); name != "" {
				out[r.Name.Name] = name
			}
		}
	}
	return out
}

// firstInput is the name of a record's first input field, or "".
func firstInput(r *syntax.RecordDecl) string {
	for _, it := range r.Body.Items {
		if f, ok := it.(*syntax.FieldDecl); ok && f.Input.Valid() {
			return f.Name.Name
		}
	}
	return ""
}

func giveInput(tg target) []progen.Site {
	holders := inputHolders(tg)
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		b, ok := f.Default.(*syntax.BraceLit)
		return ok && holders[text(tg, f.Type)] != "" && len(b.Items) == 0
	}, func(f *syntax.FieldDecl) progen.Site {
		s, _ := span(tg, f.Default)
		return seq(1, insert(s+1, " "), insert(s+1, holders[text(tg, f.Type)]), insert(s+1, `: "x" `))
	})
}

func readInput(tg target) []progen.Site {
	var out []progen.Site
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		name := inputHolders(tg)[r.Name.Name]
		if name == "" {
			continue
		}
		_, e := span(tg, r.Body.Items[len(r.Body.Items)-1])
		ind := indent(tg, e)
		out = append(out, seq(1, insert(e, "\n\n"+ind+"check "), insert(e, name), insert(e, ` != none else "no key"`)))
	}
	return out
}

// fieldOfType adds before+focus to every field of type typ without annotations.
func fieldOfType(typ, before, focus string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(f *syntax.FieldDecl) bool {
			return isSource(tg) && len(f.Annotations) == 0 && strings.HasPrefix(text(tg, f.Type), typ) && !strings.HasPrefix(text(tg, f.Type), typ+"?")
		}, func(f *syntax.FieldDecl) progen.Site {
			_, e := span(tg, f)
			return seq(1, insert(e, before), insert(e, focus))
		})
	}
}

func twoInlineFields(tg target) []progen.Site {
	variants := map[string]bool{}
	for _, name := range declared(tg, func(d *syntax.VariantDecl) *syntax.Ident { return d.Name }) {
		variants[name] = true
	}
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		return variants[text(tg, f.Type)] && len(f.Annotations) == 0 && f.Default != nil && startsLine(tg, f)
	}, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f)
		_, ne := span(tg, f.Name)
		ind := indent(tg, s)
		return seq(2, insert(e, " @json(inline)"), insert(e, "\n"+ind+"/// Another.\n"+ind), insert(e, "zzOther"),
			insert(e, string(tg.src[ne:e])+" @json(inline)"))
	})
}

// doubleOptional writes `T??` for a field's `T?`, never at a line's end (GRAMMAR.md §3.1 rule 2).
func doubleOptional(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		_, opt := f.Type.(*syntax.OptionalType)
		return isSource(tg) && opt && !endsLine(tg, f.Type)
	}, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Type)
		return site(replace(s, e, text(tg, f.Type)+"?"))
	})
}

// forceParameter asserts the presence of a use of a parameter whose type is not optional.
func forceParameter(tg target) []progen.Site {
	var out []progen.Site
	for _, f := range nodes[*syntax.FnDecl](tg) {
		if f.Mods != nil && f.Mods.Export.Valid() {
			continue // a translated function refuses "!" outright (E9001)
		}
		plain := map[string]bool{}
		for _, p := range f.Params {
			if !strings.Contains(text(tg, p.Type), "?") {
				plain[p.Name.Name] = true
			}
		}
		if f.Body == nil {
			continue
		}
		syntax.Inspect(f.Body, func(n syntax.Node) bool {
			if id, ok := n.(*syntax.IdentExpr); ok && plain[id.Name] {
				s, e := span(tg, id)
				out = append(out, site(replace(s, e, id.Name+"!")))
			}
			return true
		})
	}
	return out
}
