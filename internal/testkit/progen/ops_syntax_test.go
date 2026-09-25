package progen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on the lexer and the parser (GRAMMAR.md §1–§6, §8, §9).
func syntaxOperators() []operator {
	return []operator{
		op(diag.E1101.Def().Code, "GRAMMAR.md §2.6 (format spec)", badFormatSpec),
		op(diag.E1102.Def().Code, "GRAMMAR.md §2.6 (indentation mixes tabs and spaces)", appendSite("local const MIXED = \"\"\"\n \tline\n", " \t", "\"\"\"")),
		op(diag.E1103.Def().Code, "GRAMMAR.md §5.9 (second typeArgs)", secondTypeArgs),
		op(diag.E1104.Def().Code, "GRAMMAR.md §8.2 (unknown annotation)", fieldAnnotation(" @", "zzunknown", "")),
		op(diag.E1105.Def().Code, "GRAMMAR.md §5.10 (fail outside check)", fnStatement(`fail(1, "m")`)),
		op(diag.E1106.Def().Code, "GRAMMAR.md §1 (unexpected character)", strayCharacter),
		op(diag.E1107.Def().Code, "GRAMMAR.md §2.6 (unterminated string)", unterminatedString),
		op(diag.E1108.Def().Code, "GRAMMAR.md §2.2 (unterminated block comment)", unterminatedComment),
		op(diag.E1109.Def().Code, "GRAMMAR.md §2.6 (invalid escape)", stringPrefix(`\q`)),
		op(diag.E1110.Def().Code, "GRAMMAR.md §2.4 (leading zero)", leadingZero),
		op(diag.E1111.Def().Code, "GRAMMAR.md §2.5 (unit order)", durationOrder),
		op(diag.E1112.Def().Code, "GRAMMAR.md §2.6 (empty interpolation)", stringPrefix("{}")),
		op(diag.E1113.Def().Code, "GRAMMAR.md §2.7 (unterminated regex)", unterminatedRegex),
		op(diag.E1114.Def().Code, "GRAMMAR.md §2.7 (not RE2)", invalidRegex),
		op(diag.E1115.Def().Code, "GRAMMAR.md §2.7 (regex as an argument)", regexArgument),
		op(diag.E1116.Def().Code, "GRAMMAR.md §12 (expected IDENT)", testStatement("let ", "=", " 1")),
		op(diag.E1117.Def().Code, "GRAMMAR.md §3.1 (two items on one line)", joinStatements),
		op(diag.E1118.Def().Code, "GRAMMAR.md §8.1 (annotation position)", letPrefix(`@json("x")`, "\n")),
		op(diag.E1119.Def().Code, "GRAMMAR.md §8.2 (@since below 1)", fieldAnnotation(" ", "@since(0)", "")),
		op(diag.E1120.Def().Code, "GRAMMAR.md §8.1 (annotation twice)", fieldAnnotation(" @since(1) ", "@since(1)", "")),
		op(diag.E1121.Def().Code, "GRAMMAR.md §6.7 (argument twice)", namedArgumentTwice),
		op(diag.E1122.Def().Code, "GRAMMAR.md §2.6 (text after opening quotes)", multilineOpening),
		op(diag.E1123.Def().Code, "GRAMMAR.md §1 (byte order mark)", byteOrderMark),
		op(diag.E1124.Def().Code, "GRAMMAR.md §1 (control character in a comment)", commentControl),
		op(diag.E1125.Def().Code, "GRAMMAR.md §4.3 (reserved import alias)", reservedAlias),
		op(diag.E1126.Def().Code, "GRAMMAR.md §4.3 (none as enum member)", noneMember),
		op(diag.E1127.Def().Code, "GRAMMAR.md §5.2 (import after a declaration)", lateImport),
		op(diag.E1128.Def().Code, "GRAMMAR.md §5.11 (chained comparison)", chainedComparison),
		// DECISIONS 214: a misplaced construct keeps its node; "{}"'s own E3002 is no cascade
		{code: diag.E1129.Def().Code, rule: "GRAMMAR.md §6.1 (brace literal in a header)", also: []diag.Code{diag.E3002.Def().Code}, sites: braceInHeader},
		op(diag.E1130.Def().Code, "GRAMMAR.md §5.10 (expect outside a test)", fnStatement("expect true")),
		op(diag.E1131.Def().Code, "GRAMMAR.md §5.4 (from env without input)", fromEnvWithoutInput),
		op(diag.E1132.Def().Code, "GRAMMAR.md §2.6 (interpolated test name)", interpolatedTestName),
		op(diag.E1133.Def().Code, "GRAMMAR.md §5.3 (export on a let)", letPrefix("export", " ")),
		op(diag.E1134.Def().Code, "GRAMMAR.md §5.10 (break outside a loop)", fnStatement("break")),
		op(diag.E1135.Def().Code, "GRAMMAR.md §5.10 (return in a test)", testStatement("", "return", "")),
		op(diag.E1136.Def().Code, "GRAMMAR.md §5.11 (comprehension with two items)", twoItemComprehension),
		op(diag.E1137.Def().Code, "GRAMMAR.md §5.9 (keyed by on a non-list)", keyedNonList),
		op(diag.W1001.Def().Code, "GRAMMAR.md §9.1 (doc followed by a blank line)", detachedDoc),
		op(diag.W1002.Def().Code, "GRAMMAR.md §9.1 (public field without doc)", undocumentedField),
	}
}

// plainStrings are the one-line, non-raw string literals of a source file whose value is free:
// not a test name, an expect message, an emit option, an environment variable or an annotation
// argument, which must be constant (E1132, E1119) or have rules of their own (E1911, E7001).
func plainStrings(tg target) []*syntax.StringLit {
	if !isSource(tg) {
		return nil
	}
	constant := map[syntax.Node]bool{}
	for _, d := range nodes[*syntax.TestDecl](tg) {
		constant[d.Name] = true
	}
	for _, f := range nodes[*syntax.FieldDecl](tg) {
		constant[f.Env] = true
	}
	for _, a := range nodes[*syntax.AnnotationArg](tg) {
		constant[a.Value] = true
	}
	for _, e := range nodes[*syntax.ExpectStmt](tg) {
		constant[e.Message] = true
	}
	for _, d := range nodes[*syntax.EmitDecl](tg) {
		syntax.Inspect(d.Options, func(n syntax.Node) bool {
			constant[n] = true
			return true
		})
	}
	var out []*syntax.StringLit
	for _, s := range nodes[*syntax.StringLit](tg) {
		if !s.Multiline && !constant[s] {
			out = append(out, s)
		}
	}
	return out
}

// stringPrefix inserts text at the start of a plain string's content.
func stringPrefix(text string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, s := range plainStrings(tg) {
			start, _ := span(tg, s)
			out = append(out, site(insert(start+1, text)))
		}
		return out
	}
}

func unterminatedString(tg target) []progen.Site {
	var out []progen.Site
	for _, s := range plainStrings(tg) {
		start, end := span(tg, s)
		if len(s.Parts) <= 1 && endsLine(tg, s) {
			out = append(out, site(mark(tg, start, start+1), replace(end-1, end, "")))
		}
	}
	return out
}

func badFormatSpec(tg target) []progen.Site {
	var out []progen.Site
	if !isSource(tg) {
		return nil
	}
	for i, t := range tg.file.Tokens {
		mid := t.Kind == syntax.TokStringMid || t.Kind == syntax.TokStringTail
		if mid && i > 0 && tg.file.Tokens[i-1].Kind != syntax.TokFormatSpec {
			out = append(out, site(insert(int(t.Start), ":q")))
		}
	}
	return out
}

func secondTypeArgs(tg target) []progen.Site {
	return sitesOf(tg, func(n *syntax.NamedType) bool { return isSource(tg) && n.Args != nil },
		func(n *syntax.NamedType) progen.Site {
			_, end := span(tg, n.Args)
			return site(insert(end, "(..=5)"))
		})
}

// fieldAnnotation appends before+focus+after to every field without annotations.
func fieldAnnotation(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && len(f.Annotations) == 0 },
			func(f *syntax.FieldDecl) progen.Site {
				_, end := span(tg, f)
				return seq(1, insert(end, before), insert(end, focus), insert(end, after))
			})
	}
}

// fnBodies are the top-level blocks of the file's functions and methods.
func fnBodies(tg target) map[*syntax.Block]bool {
	out := map[*syntax.Block]bool{}
	for _, f := range nodes[*syntax.FnDecl](tg) {
		if f.Body != nil {
			out[f.Body] = true
		}
	}
	return out
}

// fnStatement inserts stmt as the first statement of a function body.
func fnStatement(stmt string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) {
			return nil
		}
		bodies := fnBodies(tg)
		return statementSites(tg, stmt, func(b *syntax.Block) bool { return bodies[b] })
	}
}

// testStatement inserts before+focus+after as the first statement of a test block.
func testStatement(before, focus, after string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, d := range nodes[*syntax.TestDecl](tg) {
			if d.Body == nil || len(d.Body.Stmts) == 0 || !startsLine(tg, d.Body.Stmts[0]) {
				continue
			}
			s, _ := span(tg, d.Body.Stmts[0])
			out = append(out, seq(1, insert(s, before), insert(s, focus), insert(s, after+"\n"+indent(tg, s))))
		}
		return out
	}
}

func strayCharacter(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && endsLine(tg, f) },
		func(f *syntax.FieldDecl) progen.Site {
			_, end := span(tg, f)
			return seq(1, insert(end, " "), insert(end, "$"))
		})
}

func unterminatedComment(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendDecl(tg, "", "/*", " never closed")}
}

func leadingZero(tg target) []progen.Site {
	return sitesOf(tg, func(n *syntax.IntLit) bool {
		t := text(tg, n)
		return isSource(tg) && strings.Trim(t, "0123456789_") == "" && !strings.HasPrefix(t, "0")
	}, func(n *syntax.IntLit) progen.Site {
		s, _ := span(tg, n)
		return site(insert(s, "0"))
	})
}

func durationOrder(tg target) []progen.Site {
	return sitesOf(tg, func(n *syntax.DurationLit) bool { return isSource(tg) && !strings.HasPrefix(text(tg, n), "-") },
		func(n *syntax.DurationLit) progen.Site {
			s, e := span(tg, n)
			return site(replace(s, e, text(tg, n)+"1d"))
		})
}

func unterminatedRegex(tg target) []progen.Site {
	return sitesOf(tg, func(*syntax.RegexLit) bool { return isSource(tg) }, func(n *syntax.RegexLit) progen.Site {
		s, e := span(tg, n)
		return site(mark(tg, s, s+1), replace(e-1, e, "\n"+indent(tg, s)))
	})
}

func invalidRegex(tg target) []progen.Site {
	return sitesOf(tg, func(*syntax.RegexLit) bool { return isSource(tg) }, func(n *syntax.RegexLit) progen.Site {
		s, e := span(tg, n)
		return site(replace(s, e, "/a(b/"))
	})
}

func regexArgument(tg target) []progen.Site {
	return sitesOf(tg, func(c *syntax.CallExpr) bool {
		sel, isSel := c.Fun.(*syntax.SelectorExpr)
		return isSource(tg) && len(c.Args) > 0 && c.Args[0].Name == nil && !(isSel && sel.Name.Name == "matches")
	}, func(c *syntax.CallExpr) progen.Site {
		s, e := span(tg, c.Args[0])
		return site(replace(s, e, "/a/"))
	})
}

// joinStatements puts the second statement of a test block on the first one's line.
func joinStatements(tg target) []progen.Site {
	var out []progen.Site
	for _, d := range nodes[*syntax.TestDecl](tg) {
		if d.Body == nil || len(d.Body.Stmts) < 2 {
			continue
		}
		_, e1 := span(tg, d.Body.Stmts[0])
		s2, e2 := span(tg, d.Body.Stmts[1])
		if !startsLine(tg, d.Body.Stmts[1]) || strings.Contains(string(tg.src[e1:s2]), "//") {
			continue
		}
		out = append(out, seq(1, replace(e1, s2, " "), mark(tg, s2, e2)))
	}
	return out
}

// letPrefix puts focus+sep before every top-level let without modifiers or annotations.
func letPrefix(focus, sep string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		return sitesOf(tg, func(d *syntax.LetDecl) bool {
			return isSource(tg) && len(d.Annotations) == 0 && (d.Mods == nil || d.Mods.Last() < d.Mods.First())
		}, func(d *syntax.LetDecl) progen.Site {
			s, _ := span(tg, d)
			return seq(0, insert(s, focus), insert(s, sep))
		})
	}
}

func namedArgumentTwice(tg target) []progen.Site {
	var out []progen.Site
	for _, c := range nodes[*syntax.CallExpr](tg) {
		for _, a := range c.Args {
			if a.Name == nil || !isSource(tg) {
				continue
			}
			_, e := span(tg, a)
			out = append(out, keeping(tg, seq(1, insert(e, ", "), insert(e, a.Name.Name), insert(e, text(tg, a)[len(a.Name.Name):])), a))
		}
	}
	return out
}

func multilineOpening(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendDecl(tg, `local const MULTI = """ `, "oops", "\n  x\n  \"\"\"")}
}

func byteOrderMark(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{site(insert(0, "\uFEFF"))}
}

func reservedAlias(tg target) []progen.Site {
	return sitesOf(tg, func(i *syntax.Import) bool { return i.Alias == nil }, func(i *syntax.Import) progen.Site {
		_, e := span(tg, i.Path)
		return seq(1, insert(e, " as "), insert(e, "let"))
	})
}

// noneMember appends a member "none"; in a @codes enum, with a free code of its type, since the
// member still exists for checking and a missing or clashing code is a finding of its own
// (decision log "Check C2 — calls").
func noneMember(tg target) []progen.Site {
	var out []progen.Site
	for _, d := range nodes[*syntax.EnumDecl](tg) {
		if !isSource(tg) || len(d.Members) == 0 {
			continue
		}
		_, e := span(tg, d.Members[len(d.Members)-1])
		s := seq(1, insert(e, ", "), insert(e, "none"))
		if ce, coded := codedOf(tg, d); coded {
			code, ok := freeCode(ce)
			if !ok {
				continue
			}
			s.Edits = append(s.Edits, insert(e, " = "+code))
		}
		out = append(out, s)
	}
	return out
}

func lateImport(tg target) []progen.Site {
	if !isSource(tg) || len(tg.file.Decls) == 0 {
		return nil
	}
	return []progen.Site{appendDecl(tg, "", "import", " sovcommon.roles")}
}

// chainedComparison chains "== true" onto a comparison, never onto a test against none, whose
// narrowing the chain would undo (E3402, E3403 of the mutant's own making).
func chainedComparison(tg target) []progen.Site {
	return sitesOf(tg, func(b *syntax.BinaryExpr) bool {
		return isSource(tg) && (b.Op == syntax.TokEq || b.Op == syntax.TokNe || b.Op == syntax.TokLt || b.Op == syntax.TokGe) && !narrows(b)
	}, func(b *syntax.BinaryExpr) progen.Site {
		_, e := span(tg, b)
		return seq(1, insert(e, " "), insert(e, "=="), insert(e, " true"))
	})
}

// braceInHeader compares an if's condition, parenthesized, with {}: the condition may itself be
// a comparison, which a bare "== {}" would chain (E1128); never a condition that narrows.
func braceInHeader(tg target) []progen.Site {
	return sitesOf(tg, func(s *syntax.IfStmt) bool { return s.Cond != nil && !narrows(s.Cond) }, func(s *syntax.IfStmt) progen.Site {
		b, e := span(tg, s.Cond)
		return seq(3, insert(b, "("), insert(e, ")"), insert(e, " == "), insert(e, "{}"))
	})
}

func fromEnvWithoutInput(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && f.Default != nil && !f.Input.Valid() },
		func(f *syntax.FieldDecl) progen.Site {
			_, e := span(tg, f.Type)
			return seq(1, insert(e, " "), insert(e, "from"), insert(e, ` env "X"`))
		})
}

func interpolatedTestName(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.TestDecl) bool { return d.Name != nil }, func(d *syntax.TestDecl) progen.Site {
		s, _ := span(tg, d.Name)
		return site(mark(tg, s, s+1), insert(s+1, "{x}"))
	})
}

func twoItemComprehension(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	return []progen.Site{appendDecl(tg, `local let zzComp: {String: Int} = { "a": 1, `, `"b": 2`, " for x in [1] }")}
}

func keyedNonList(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.LetDecl) bool {
		_, named := d.Type.(*syntax.NamedType)
		return isSource(tg) && named
	}, func(d *syntax.LetDecl) progen.Site {
		_, e := span(tg, d.Type)
		return seq(1, insert(e, " "), insert(e, "keyed"), insert(e, " by id"))
	})
}

func detachedDoc(tg target) []progen.Site {
	return sitesOf(tg, func(d *syntax.LetDecl) bool { return isSource(tg) && d.Doc != nil }, func(d *syntax.LetDecl) progen.Site {
		return site(mark(tg, int(d.Doc.Start), int(d.Doc.Start)+3), insert(int(d.Doc.End), "\n"))
	})
}

func undocumentedField(tg target) []progen.Site {
	return sitesOf(tg, func(f *syntax.FieldDecl) bool { return isSource(tg) && f.Doc != nil }, func(f *syntax.FieldDecl) progen.Site {
		s, e := span(tg, f.Name)
		return site(mark(tg, s, e), replace(lineStart(tg, int(f.Doc.Start)), lineStart(tg, s), ""))
	})
}

// commentControl puts a control character at the start of a line comment's text.
func commentControl(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	var out []progen.Site
	for _, t := range tg.file.Tokens {
		for _, tr := range append(append([]syntax.Trivia(nil), t.Leading...), t.Trailing...) {
			if tr.Kind == syntax.TriviaLineComment {
				out = append(out, site(insert(int(tr.Start)+len("//"), "\x01")))
			}
		}
	}
	return out
}

// narrows tells an expression holding a test that narrows an optional or a variant: a comparison
// with none, or `is`.
func narrows(e syntax.Node) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		switch v := n.(type) {
		case *syntax.IsExpr:
			found = true
		case *syntax.BinaryExpr:
			_, x := v.X.(*syntax.NoneLit)
			_, y := v.Y.(*syntax.NoneLit)
			found = found || x || y
		}
		return !found
	})
	return found
}
