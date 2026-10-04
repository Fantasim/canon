package progen_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on typing (TYPES.md §5–§15): each breaks one expression, type or declaration.
func typesOperators() []operator {
	return []operator{
		op(diag.E3001.Def().Code, "TYPES.md §15 (public let without a type)", appendSite("/// Untyped.\nlet ", "zzUntyped", " = 1")),
		op(diag.E3002.Def().Code, "TYPES.md §6.2 (Int for a String field)", literalFields(isString, "1")),
		op(diag.E3003.Def().Code, "TYPES.md §3.5 (unknown member)", unknownMember),
		op(diag.E3004.Def().Code, "TYPES.md §12.2 (too many arguments)", extraArgument),
		op(diag.E3005.Def().Code, "TYPES.md §12.2 (calling a constant)", callConstant),
		op(diag.E3006.Def().Code, "TYPES.md §12.1 (finish without returning)", guardLastReturn),
		op(diag.E3007.Def().Code, "TYPES.md §7.1 (arithmetic on Bool)", arithmeticOnBool),
		op(diag.E3008.Def().Code, "TYPES.md §5.3 (none alone)", fnStatementFocus("let zzNone = ", "none", "")),
		op(diag.E3010.Def().Code, "TYPES.md §15 (default reading a let)", defaultReadsLet),
		op(diag.E3011.Def().Code, "TYPES.md §9.2 (Bool map key)", fnStatementFocus("let zzMap: {", "Bool", ": Int} = {}")),
		op(diag.E3012.Def().Code, "TYPES.md §9.1 (keyed by no field)", keyedByUnknown),
		op(diag.E3013.Def().Code, "TYPES.md §9.3 (table of an enum)", tableOfEnum),
		op(diag.E3015.Def().Code, "TYPES.md §15 (refinement bound reads a let)", boundReadsLet),
		op(diag.E3016.Def().Code, "TYPES.md §12.3 (built-in as a value)", fnStatementFocus("let zzAbs: fn(Int) -> Int = ", "abs", "")),
		op(diag.E3017.Def().Code, "TYPES.md §12.7 (assign to a let)", assignLet),
		op(diag.E3018.Def().Code, "TYPES.md §12.7 (two names over a list)", twoNamesOverList),
		op(diag.E3019.Def().Code, "TYPES.md §12.1 (bare return)", bareReturn),
		// TYPES.md §5.1 (a bare "1 + 2" has no context, so E3008 pre-empts E3020: progen artifact)
		op(diag.E3020.Def().Code, "TYPES.md §12.7 (expression without effect)", fnStatementFocus("", "true", "")),
		op(diag.E3021.Def().Code, "TYPES.md §13.1 (alias refers to itself)", selfAlias),
		op(diag.E3022.Def().Code, "TYPES.md §13.1 (record contains itself)", selfRecord("")),
		op(diag.E3022.Def().Code, "TYPES.md §13.1 (record contains itself, with a default)", selfRecordDefault),
		op(diag.E3023.Def().Code, "TYPES.md §7.4 (range on Bool)", rangeOnBool),
		op(diag.E3025.Def().Code, "TYPES.md §13.3 (Float bound in a Range)", fnStatementFocus("let zzR: Range = ", "1.5..3", "")),
		op(diag.E3101.Def().Code, "TYPES.md §9.3 (duplicate table key)", duplicateTableEntry),
		{code: diag.E3102.Def().Code, rule: "TYPES.md §9.1 (@codes code twice)", also: []diag.Code{diag.E6002.Def().Code}, sites: codeTwice},
		op(diag.E3103.Def().Code, "TYPES.md §9.3 (entry of a non-table)", entryOfNonTable),
		op(diag.E3201.Def().Code, "TYPES.md §7.2 (code outside its @codes type)", codeOutOfRange),
		op(diag.E3204.Def().Code, "TYPES.md §7.4 (default below its range)", defaultBelowRange),
		op(diag.E3205.Def().Code, "TYPES.md §7.4 (literal not matching its pattern)", patternMismatch),
	}
}

// appendSite appends before+focus+after to every source file, but those of packages with a
// data-mode go emit, whose rules (E8015, E8153) a new public value would meet.
func appendSite(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) || dataMode(tg) {
			return nil
		}
		return []progen.Site{appendDecl(tg, before, focus, after)}
	}
}

// fnStatementFocus inserts before+focus+after as the first statement of each function body.
func fnStatementFocus(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) {
			return nil
		}
		var out []progen.Site
		for _, f := range nodes[*syntax.FnDecl](tg) {
			if f.Body == nil || len(f.Body.Stmts) == 0 || !startsLine(tg, f.Body.Stmts[0]) {
				continue
			}
			s, _ := span(tg, f.Body.Stmts[0])
			out = append(out, seq(1, insert(s, before), insert(s, focus), insert(s, after+"\n"+indent(tg, s))))
		}
		return out
	}
}

// valueLiterals are the brace literals of the values of top-level lets.
func valueLiterals(tg target) []*syntax.BraceLit {
	var out []*syntax.BraceLit
	for _, d := range nodes[*syntax.LetDecl](tg) {
		if d.Value == nil {
			continue
		}
		syntax.Inspect(d.Value, func(n syntax.Node) bool {
			if b, ok := n.(*syntax.BraceLit); ok {
				out = append(out, b)
			}
			return true
		})
	}
	return out
}

// isString tells a string literal: the values the E3002 operator replaces with an Int.
func isString(v syntax.Expr) bool { _, ok := v.(*syntax.StringLit); return ok }

// literalFields replaces each let literal item keep accepts, but a dependent field's (TYPES.md §11.4).
func literalFields(keep func(syntax.Expr) bool, with string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		dependent := dependentFields(tg)
		for _, b := range valueLiterals(tg) {
			for _, it := range b.Items {
				if f, ok := it.(*syntax.FieldItem); ok && keep(f.Value) && !dependent[f.Name.Name] {
					s, e := span(tg, f.Value)
					out = append(out, site(replace(s, e, with)))
				}
			}
		}
		return out
	}
}

// dependentFields are the names of tg's package's fields whose type applies a type function.
func dependentFields(tg target) map[string]bool {
	fns := map[string]bool{}
	for _, other := range *tg.all {
		for _, d := range nodes[*syntax.TypeDecl](other) {
			fns[d.Name.Name] = fns[d.Name.Name] || len(d.Params) > 0
		}
	}
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			out[f.Name.Name] = out[f.Name.Name] || applies(f.Type, fns)
		}
	}
	return out
}

// applies tells whether n applies one of the type functions fns.
func applies(n syntax.Node, fns map[string]bool) bool {
	return holds(n, func(c syntax.Node) bool {
		t, ok := c.(*syntax.NamedType)
		return ok && t != nil && t.Args != nil && fns[t.Name.Parts[len(t.Name.Parts)-1].Name]
	})
}

// unknownMember renames a member, never a key of a table or keyed list (TYPES.md §3.5).
func unknownMember(tg target) []progen.Site {
	keyed := collections(tg)
	return sitesOf(tg, func(s *syntax.SelectorExpr) bool {
		return isSource(tg) && !s.Optional && !keyed[lastName(s.X)]
	}, func(s *syntax.SelectorExpr) progen.Site {
		ns, ne := span(tg, s.Name)
		return site(replace(ns, ne, "zz"+s.Name.Name))
	})
}

// variadic are the built-ins that take any number of arguments (STDLIB.md §2.2).
var variadic = map[string]bool{"min": true, "max": true}

func extraArgument(tg target) []progen.Site {
	return sitesOf(tg, func(c *syntax.CallExpr) bool {
		fn, isName := c.Fun.(*syntax.IdentExpr)
		return isSource(tg) && len(c.Args) > 0 && c.Args[len(c.Args)-1].Name == nil && !(isName && variadic[fn.Name])
	},
		func(c *syntax.CallExpr) progen.Site {
			s, _ := span(tg, c)
			_, e := span(tg, c.Args[len(c.Args)-1])
			return seq(0, replace(s, e, string(tg.src[s:e])+", 1"))
		})
}

func callConstant(tg target) []progen.Site {
	consts := map[string]bool{}
	for _, name := range declared(tg, func(d *syntax.ConstDecl) *syntax.Ident { return d.Name }) {
		consts[name] = true
	}
	return sitesOf(tg, func(id *syntax.IdentExpr) bool { return consts[id.Name] }, func(id *syntax.IdentExpr) progen.Site {
		s, e := span(tg, id)
		return site(mark(tg, s, e), insert(e, "(1)"))
	})
}

func guardLastReturn(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FnDecl) bool {
		if f.Body == nil || len(f.Body.Stmts) == 0 {
			return false
		}
		_, ok := f.Body.Stmts[len(f.Body.Stmts)-1].(*syntax.ReturnStmt)
		return ok
	}, func(f *syntax.FnDecl) progen.Site {
		s, e := span(tg, f.Body.Stmts[len(f.Body.Stmts)-1])
		ns, ne := span(tg, f.Name)
		return site(mark(tg, ns, ne), replace(s, e, "if true { "+string(tg.src[s:e])+" }"))
	})
}

func arithmeticOnBool(tg target) []progen.Site {
	arith := map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true, syntax.TokStar: true, syntax.TokPercent: true}
	return sitesOf(tg, func(b *syntax.BinaryExpr) bool { return isSource(tg) && arith[b.Op] }, func(b *syntax.BinaryExpr) progen.Site {
		s, _ := span(tg, b)
		ys, ye := span(tg, b.Y)
		return seq(0, mark(tg, s, s), replace(ys, ye, "true"))
	})
}

// publicLets are the names of tg's package's public top-level lets.
func publicLets(tg target) []string {
	var out []string
	for _, p := range peers(tg) {
		for _, d := range nodes[*syntax.LetDecl](p) {
			if d.Mods == nil || !d.Mods.Local.Valid() {
				out = append(out, d.Name.Name)
			}
		}
	}
	return out
}

// defaultReadsLet moves a field's default into a new local let of the field's type, and makes
// the default read that let: the value is the same, only what the default reads is wrong. Never a
// type or default reading a field or a parameter, which the let would not see.
func defaultReadsLet(tg target) []progen.Site {
	scope := recordScope(tg)
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		return isSource(tg) && f.Default != nil && !reads(f.Type, scope) && !reads(f.Default, scope)
	}, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Default)
		let := "\n\nlocal let zzDefault: " + text(tg, f.Type) + " = " + text(tg, f.Default) + "\n"
		return site(replace(s, e, "zzDefault"), insert(declEnd(tg), let))
	})
}

// recordScope are the names of tg's fields and parameters: what a field's type or default may
// read inside its record and a top-level let cannot.
func recordScope(tg target) map[string]bool {
	out := map[string]bool{}
	for _, f := range nodes[*syntax.FieldDecl](tg) {
		out[f.Name.Name] = true
	}
	for _, p := range nodes[*syntax.Param](tg) {
		out[p.Name.Name] = true
	}
	return out
}

// reads tells whether an expression of n reads one of names.
func reads(n syntax.Node, names map[string]bool) bool {
	return holds(n, func(c syntax.Node) bool {
		id, ok := c.(*syntax.IdentExpr)
		return ok && id != nil && names[id.Name]
	})
}

func keyedByUnknown(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.LetDecl) bool {
		_, list := d.Type.(*syntax.ListType)
		return isSource(tg) && list
	}, func(d *syntax.LetDecl) progen.Site {
		_, e := span(tg, d.Type)
		return seq(1, insert(e, " keyed by "), insert(e, "zzid"))
	})
}

// referenced are the type names a ref of tg's package names.
func referenced(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, r := range nodes[*syntax.RefType](p) {
			out[text(p, r.Name)] = true
		}
	}
	return out
}

// tableOfEnum makes a table hold an enum, never one of a record with a @stable field (LOCK.md §1).
func tableOfEnum(tg target) []progen.Site {
	var out []progen.Site
	refs := referenced(tg)
	stable := stableFielded(tg)
	for _, enum := range declared(tg, func(d *syntax.EnumDecl) *syntax.Ident { return d.Name }) {
		for _, t := range nodes[*syntax.TableType](tg) {
			if refs[text(tg, t.Name)] || stable[text(tg, t.Name)] {
				continue
			}
			s, e := span(tg, t.Name)
			out = append(out, site(replace(s, e, enum)))
		}
	}
	return out
}

// stableFielded are the records of tg's package with a @stable field.
func stableFielded(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, r := range nodes[*syntax.RecordDecl](p) {
			out[r.Name.Name] = r.Body != nil && slices.ContainsFunc(r.Body.Items, func(it syntax.RecordItem) bool {
				f, ok := it.(*syntax.FieldDecl)
				return ok && annotated(f.Annotations, annotStable)
			})
		}
	}
	return out
}

func boundReadsLet(tg target) []progen.Site {
	return sitesOf(tg, func(r *syntax.RangeExpr) bool {
		_, isInt := r.Hi.(*syntax.IntLit)
		return isSource(tg) && isInt
	}, func(r *syntax.RangeExpr) progen.Site {
		s, e := span(tg, r.Hi)
		end := declEnd(tg)
		return site(replace(s, e, "zzLimit"), insert(end, "\n\nlocal let zzLimit: Int = 3\n"))
	})
}

func assignLet(tg target) []progen.Site {
	return sitesOf(tg, func(l *syntax.LetStmt) bool { return startsLine(tg, l) && endsLine(tg, l) }, func(l *syntax.LetStmt) progen.Site {
		s, e := span(tg, l)
		at := lineEnd(tg, e) + 1
		return seq(1, insert(at, indent(tg, s)), insert(at, l.Name.Name), insert(at, " = "+l.Name.Name+"\n"))
	})
}

func twoNamesOverList(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.ForStmt) bool { return len(f.Vars) == 1 }, func(f *syntax.ForStmt) progen.Site {
		_, ve := span(tg, f.Vars[0])
		is, ie := span(tg, f.Iter)
		return seq(1, insert(ve, ", zzB"), mark(tg, is, ie))
	})
}

func bareReturn(tg target) []progen.Site {
	return sitesOf(tg, func(r *syntax.ReturnStmt) bool { return r.Value != nil }, func(r *syntax.ReturnStmt) progen.Site {
		s, e := span(tg, r)
		return site(replace(s, e, "return"))
	})
}

func selfAlias(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.TypeDecl) bool { return d.Type != nil && len(d.Params) == 0 }, func(d *syntax.TypeDecl) progen.Site {
		ns, ne := span(tg, d.Name)
		s, e := span(tg, d.Type)
		return site(mark(tg, ns, ne), replace(s, e, d.Name.Name))
	})
}

// selfRecordDefault adds "zzSelf: R = {}" to every record R whose fields all have defaults, so
// that "{}" is a whole R but for the new field.
func selfRecordDefault(tg target) []progen.Site {
	var out []progen.Site
	for _, s := range selfRecord(" = {}")(tg) {
		if allDefaulted(tg, s.Edits[0].Text) {
			out = append(out, s)
		}
	}
	return out
}

// allDefaulted tells a record of tg, by name, whose every field has a default or is optional.
func allDefaulted(tg target, name string) bool {
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		if r.Name.Name != name {
			continue
		}
		for _, it := range r.Body.Items {
			if f, ok := it.(*syntax.FieldDecl); ok && f.Default == nil && !f.Input.Valid() {
				return false
			}
		}
		return true
	}
	return false
}

// selfRecord adds a field of its own type, then after, to each record without input fields (DECISIONS 214).
func selfRecord(after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(r *syntax.RecordDecl) bool {
			return r.Body != nil && len(r.Body.Items) > 0 && len(r.Params) == 0 && !hasInputField(r)
		}, func(r *syntax.RecordDecl) progen.Site {
			ns, ne := span(tg, r.Name)
			_, e := span(tg, r.Body.Items[len(r.Body.Items)-1])
			ind := indent(tg, e)
			return site(mark(tg, ns, ne), insert(e, "\n"+ind+"/// Itself.\n"+ind+"zzSelf: "+r.Name.Name+after))
		})
	}
}

// hasInputField tells a record with an `input` field (TYPES.md §14).
func hasInputField(r *syntax.RecordDecl) bool {
	return slices.ContainsFunc(r.Body.Items, func(it syntax.RecordItem) bool {
		f, ok := it.(*syntax.FieldDecl)
		return ok && f.Input.Valid()
	})
}

func rangeOnBool(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && text(tg, f.Type) == "Bool" },
		func(f *syntax.FieldDecl) progen.Site {
			_, e := span(tg, f.Type)
			return seq(1, insert(e, "("), insert(e, "1.."), insert(e, ")"))
		})
}

func duplicateTableEntry(tg target) []progen.Site {
	return sitesOf(tg, func(it *syntax.EntryItem) bool { return startsLine(tg, it) && endsLine(tg, it) && it.Mods == nil },
		func(it *syntax.EntryItem) progen.Site {
			s, e := span(tg, it)
			at := lineEnd(tg, e) + 1
			return keeping(tg, seq(1, insert(at, indent(tg, s)), insert(at, it.Key.Name), insert(at, string(tg.src[s+len(it.Key.Name):e])+"\n")), it.Key)
		})
}

// codedEnum is an enum with @codes: its codes type and the members with an integer value.
type codedEnum struct {
	typ     string
	members []*syntax.EnumMember
}

// codedEnums are tg's enums with @codes.
func codedEnums(tg target) []codedEnum {
	var out []codedEnum
	for _, d := range nodes[*syntax.EnumDecl](tg) {
		if ce, ok := codedOf(tg, d); ok {
			out = append(out, ce)
		}
	}
	return out
}

// codedOf is d as a codedEnum, false when d has no @codes.
func codedOf(tg target, d *syntax.EnumDecl) (codedEnum, bool) {
	typ, ok := codesType(tg, d)
	if !ok {
		return codedEnum{}, false
	}
	ce := codedEnum{typ: typ}
	for _, m := range d.Members {
		if _, ok := m.Value.(*syntax.IntLit); ok {
			ce.members = append(ce.members, m)
		}
	}
	return ce, true
}

// codesType is the type an enum's @codes annotation names (GRAMMAR.md §8), false without one.
func codesType(tg target, d *syntax.EnumDecl) (string, bool) {
	for _, a := range d.Annotations {
		if a.Name.Name == "codes" && len(a.Args) == 1 {
			return text(tg, a.Args[0]), true
		}
	}
	return "", false
}

func codeTwice(tg target) []progen.Site {
	var out []progen.Site
	for _, ce := range codedEnums(tg) {
		ms := ce.members
		for i := 1; i < len(ms); i++ {
			s, e := span(tg, ms[i].Value)
			ns, _ := span(tg, ms[i])
			out = append(out, keeping(tg, seq(0, mark(tg, ns, s), replace(s, e, text(tg, ms[0].Value))), ms[0].Value))
		}
	}
	return out
}

// codesBits is, per @codes type, the exponent of the least power of two past its range (TYPES.md §7.2).
var codesBits = map[string]int{
	"Int8": 7, "UInt8": 8, "Int16": 15, "UInt16": 16, "Int32": 31, "UInt32": 32, "Int": maxCodeBits, "UInt64": maxCodeBits,
}

// definesCall starts a let's value that is a load.defines table.
const definesCall = "load.defines("

// maxCodeBits is 63: 2^63 is past every integer literal (GRAMMAR.md §2.4).
const maxCodeBits = 63

// codeOutOfRange writes a code its enum's @codes type cannot hold, in the enums whose type a
// literal can overflow.
func codeOutOfRange(tg target) []progen.Site {
	var out []progen.Site
	for _, ce := range codedEnums(tg) {
		bits, ok := codesBits[ce.typ]
		if !ok || bits >= maxCodeBits {
			continue
		}
		past := strconv.FormatUint(1<<bits, decimalBase)
		for _, m := range ce.members {
			s, e := span(tg, m.Value)
			out = append(out, site(replace(s, e, past)))
		}
	}
	return out
}

// freeCode is the least power of two ce's type holds that no member of ce uses (so a member
// written with it meets neither E3102, E3201 nor @json(bits)); false when there is none.
func freeCode(ce codedEnum) (string, bool) {
	used := map[string]bool{}
	for _, m := range ce.members {
		used[m.Value.(*syntax.IntLit).Value.String()] = true
	}
	for k := range codesBits[ce.typ] {
		if code := strconv.FormatUint(1<<k, decimalBase); !used[code] {
			return code, true
		}
	}
	return "", false
}

// collections are the names of the corpus's lets typed as a table or a keyed list, or holding a
// load.defines table (STDLIB.md: `table Define`), whose members are keys.
func collections(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range *tg.all {
		for _, d := range nodes[*syntax.LetDecl](p) {
			t := textOr(p, d.Type)
			if strings.Contains(t, "table") || strings.Contains(t, "keyed") || strings.HasPrefix(textOr(p, d.Value), definesCall) {
				out[d.Name.Name] = true
			}
		}
	}
	return out
}

// lastName is the name an expression ends with: x for x and a.x, "" for anything else.
func lastName(e syntax.Expr) string {
	switch n := e.(type) {
	case *syntax.IdentExpr:
		return n.Name
	case *syntax.SelectorExpr:
		return n.Name.Name
	}
	return ""
}

// nonCollections are tg's package's lets whose type is neither a table nor a keyed list.
func nonCollections(tg target) []string {
	var out []string
	for _, p := range peers(tg) {
		for _, d := range nodes[*syntax.LetDecl](p) {
			if d.Type != nil && !strings.Contains(text(p, d.Type), "table") && !strings.Contains(text(p, d.Type), "keyed") {
				out = append(out, d.Name.Name)
			}
		}
	}
	return out
}

func entryOfNonTable(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	var out []progen.Site
	for _, name := range nonCollections(tg) {
		out = append(out, appendDecl(tg, "entry ", name, ".zz {}"))
	}
	return out
}

func defaultBelowRange(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		nt, ok := f.Type.(*syntax.NamedType)
		if !ok || nt.Args == nil || len(nt.Args.Args) != 1 || f.Default == nil {
			return false
		}
		r, ok := nt.Args.Args[0].(*syntax.RangeExpr)
		if !ok {
			return false
		}
		lo, isInt := r.Lo.(*syntax.IntLit)
		_, defInt := f.Default.(*syntax.IntLit)
		return isInt && defInt && lo.Value.Sign() >= 0
	}, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Default)
		return site(replace(s, e, "-1"))
	})
}

// patterned are the patterns of the package's fields refined by a regex alone, by field name.
func patterned(tg target) map[string][]*regexp.Regexp {
	out := map[string][]*regexp.Regexp{}
	for _, p := range peers(tg) {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			if re, ok := regexOf(f.Type); ok {
				out[f.Name.Name] = append(out[f.Name.Name], re)
			}
		}
	}
	return out
}

// regexOf is the pattern of a named type refined by a regex alone.
func regexOf(t syntax.Type) (*regexp.Regexp, bool) {
	nt, ok := t.(*syntax.NamedType)
	if !ok || nt.Args == nil || len(nt.Args.Args) != 1 {
		return nil, false
	}
	lit, ok := nt.Args.Args[0].(*syntax.RegexLit)
	if !ok {
		return nil, false
	}
	re, err := regexp.Compile(lit.Pattern)
	return re, err == nil
}

// patternMismatch writes, in place of a string of a pattern-refined field, the first candidate
// every pattern of that field name refuses (an RE2 search, STD-03).
func patternMismatch(tg target) []progen.Site {
	fields := patterned(tg)
	var out []progen.Site
	for _, b := range valueLiterals(tg) {
		for _, it := range b.Items {
			f, ok := it.(*syntax.FieldItem)
			if !ok || len(fields[f.Name.Name]) == 0 {
				continue
			}
			bad, found := refused(fields[f.Name.Name])
			if _, str := f.Value.(*syntax.StringLit); str && found {
				s, e := span(tg, f.Value)
				out = append(out, site(replace(s, e, strconv.Quote(bad))))
			}
		}
	}
	return out
}

// mismatches are the strings patternMismatch tries, in order.
var mismatches = []string{"zz", "", "zz !?", "0"}

// refused is the first of mismatches no pattern finds a match in.
func refused(patterns []*regexp.Regexp) (string, bool) {
	for _, m := range mismatches {
		if !slices.ContainsFunc(patterns, func(re *regexp.Regexp) bool { return re.MatchString(m) }) {
			return m, true
		}
	}
	return "", false
}
