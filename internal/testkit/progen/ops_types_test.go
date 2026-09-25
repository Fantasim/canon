package progen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on typing (TYPES.md §5–§15): each breaks one expression, type or declaration.
func typesOperators() []operator {
	return []operator{
		op(diag.E3001.Def().Code, "TYPES.md §15 (public let without a type)", appendSite("/// Untyped.\nlet ", "zzUntyped", " = 1")),
		op(diag.E3002.Def().Code, "TYPES.md §6.2 (Int for a String field)", literalFields(func(v syntax.Expr) bool { _, ok := v.(*syntax.StringLit); return ok }, "1")),
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
		op(diag.E3020.Def().Code, "TYPES.md §12.7 (expression without effect)", fnStatementFocus("", "1 + 2", "")),
		op(diag.E3021.Def().Code, "TYPES.md §13.1 (alias refers to itself)", selfAlias),
		op(diag.E3022.Def().Code, "TYPES.md §13.1 (record contains itself)", selfRecord("")),
		op(diag.E3022.Def().Code, "TYPES.md §13.1 (record contains itself, with a default)", selfRecordDefault),
		op(diag.E3023.Def().Code, "TYPES.md §7.4 (range on Bool)", rangeOnBool),
		op(diag.E3025.Def().Code, "TYPES.md §13.3 (Float bound in a Range)", fnStatementFocus("let zzR: Range = ", "1.5..3", "")),
		op(diag.E3101.Def().Code, "TYPES.md §9.3 (duplicate table key)", duplicateTableEntry),
		op(diag.E3102.Def().Code, "TYPES.md §9.1 (@codes code twice)", codeTwice),
		op(diag.E3103.Def().Code, "TYPES.md §9.3 (entry of a non-table)", entryOfNonTable),
		op(diag.E3201.Def().Code, "TYPES.md §7.2 (code outside UInt8)", codeOutOfRange),
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

// literalFields replaces the value of every named item of a let's literal that keep accepts.
func literalFields(keep func(syntax.Expr) bool, with string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, b := range valueLiterals(tg) {
			for _, it := range b.Items {
				if f, ok := it.(*syntax.FieldItem); ok && keep(f.Value) {
					s, e := span(tg, f.Value)
					out = append(out, site(replace(s, e, with)))
				}
			}
		}
		return out
	}
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
// the default read that let: the value is the same, only what the default reads is wrong.
func defaultReadsLet(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && f.Default != nil }, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Default)
		let := "\n\nlocal let zzDefault: " + text(tg, f.Type) + " = " + text(tg, f.Default) + "\n"
		return site(replace(s, e, "zzDefault"), insert(declEnd(tg), let))
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

func tableOfEnum(tg target) []progen.Site {
	var out []progen.Site
	refs := referenced(tg)
	for _, enum := range declared(tg, func(d *syntax.EnumDecl) *syntax.Ident { return d.Name }) {
		for _, t := range nodes[*syntax.TableType](tg) {
			if refs[text(tg, t.Name)] {
				continue
			}
			s, e := span(tg, t.Name)
			out = append(out, site(replace(s, e, enum)))
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

// selfRecord adds to every record a documented field of its own type, then after.
func selfRecord(after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(r *syntax.RecordDecl) bool {
			return r.Body != nil && len(r.Body.Items) > 0 && len(r.Params) == 0
		}, func(r *syntax.RecordDecl) progen.Site {
			ns, ne := span(tg, r.Name)
			_, e := span(tg, r.Body.Items[len(r.Body.Items)-1])
			ind := indent(tg, e)
			return site(mark(tg, ns, ne), insert(e, "\n"+ind+"/// Itself.\n"+ind+"zzSelf: "+r.Name.Name+after))
		})
	}
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

// codedMembers are the members with an integer value of enums with @codes.
func codedMembers(tg target) [][]*syntax.EnumMember {
	var out [][]*syntax.EnumMember
	for _, d := range nodes[*syntax.EnumDecl](tg) {
		coded := false
		for _, a := range d.Annotations {
			coded = coded || a.Name.Name == "codes"
		}
		var ms []*syntax.EnumMember
		for _, m := range d.Members {
			if _, ok := m.Value.(*syntax.IntLit); ok && coded {
				ms = append(ms, m)
			}
		}
		out = append(out, ms)
	}
	return out
}

func codeTwice(tg target) []progen.Site {
	var out []progen.Site
	for _, ms := range codedMembers(tg) {
		for i := 1; i < len(ms); i++ {
			s, e := span(tg, ms[i].Value)
			ns, _ := span(tg, ms[i])
			out = append(out, keeping(tg, seq(0, mark(tg, ns, s), replace(s, e, text(tg, ms[0].Value))), ms[0].Value))
		}
	}
	return out
}

func codeOutOfRange(tg target) []progen.Site {
	var out []progen.Site
	for _, ms := range codedMembers(tg) {
		for _, m := range ms {
			s, e := span(tg, m.Value)
			out = append(out, site(replace(s, e, "300")))
		}
	}
	return out
}

// collections are the names of the corpus's lets typed as a table or a keyed list.
func collections(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range *tg.all {
		for _, d := range nodes[*syntax.LetDecl](p) {
			if t := textOr(p, d.Type); strings.Contains(t, "table") || strings.Contains(t, "keyed") {
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

// patterned are the fields of tg's package whose type is String refined by a pattern.
func patterned(tg target) map[string]bool {
	out := map[string]bool{}
	for _, p := range peers(tg) {
		for _, f := range nodes[*syntax.FieldDecl](p) {
			if hasRegex(f.Type) {
				out[f.Name.Name] = true
			}
		}
	}
	return out
}

// hasRegex tells a named type refined by a regex alone.
func hasRegex(t syntax.Type) bool {
	nt, ok := t.(*syntax.NamedType)
	if !ok || nt.Args == nil || len(nt.Args.Args) != 1 {
		return false
	}
	_, re := nt.Args.Args[0].(*syntax.RegexLit)
	return re
}

func patternMismatch(tg target) []progen.Site {
	fields := patterned(tg)
	var out []progen.Site
	for _, b := range valueLiterals(tg) {
		for _, it := range b.Items {
			f, ok := it.(*syntax.FieldItem)
			if !ok || !fields[f.Name.Name] {
				continue
			}
			if _, str := f.Value.(*syntax.StringLit); str {
				s, e := span(tg, f.Value)
				out = append(out, site(replace(s, e, `"zz"`)))
			}
		}
	}
	return out
}
