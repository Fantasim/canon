package progen_test

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on evaluation, the standard library, checks, locks and load forms.
func evalOperators() []operator {
	return []operator{
		op(diag.E4001.Def().Code, "TYPES.md §6.5 (! on none)", appendSite("local let zzN: Int? = none\nlocal let zzV: Int = ", "zzN!", "")),
		op(diag.E4002.Def().Code, "EVALUATION.md §4.1 (index out of range)", appendSite("local let zzXs: [Int] = [1, 2]\nlocal let zzY: Int = ", "zzXs[5]", "")),
		op(diag.E4101.Def().Code, "EVALUATION.md §6.1 (integer overflow)", appendSite("local let zzBig: Int = 9223372036854775807\nlocal let zzO: Int = ", "zzBig + 1", "")),
		op(diag.E4102.Def().Code, "EVALUATION.md §6.1 (division by zero)", appendSite("local let zzZero: Int = 0\nlocal let zzQ: Int = ", "7 / zzZero", "")),
		op(diag.E4103.Def().Code, "EVALUATION.md §6.2 (float past Int)", appendSite("local let zzHuge: Float = 1e300\nlocal let zzI: Int = ", "Int(zzHuge)", "")),
		op(diag.E4104.Def().Code, "EVALUATION.md §6.2 (float overflow)", appendSite("local let zzBigF: Float = 1e308\nlocal let zzInf: Float = ", "zzBigF * 10.0", "")),
		op(diag.E4105.Def().Code, "STDLIB.md §4.2 (zip lengths)", appendSite("local let zzXs: [Int] = [1, 2, 3]\nlocal let zzZ: Int = ", `zzXs.zip(["a"])`, ".len()")),
		op(diag.E4106.Def().Code, "STDLIB.md §7 (empty separator)", appendSite("local let zzEmpty: String = \"\"\nlocal let zzParts: [String] = ", `"a,b".split(zzEmpty)`, "")),
		op(diag.E4107.Def().Code, "STDLIB.md §7 (slice inside a character)", appendSite("local let zzWord: String = \"été\"\nlocal let zzHead: String = ", "zzWord[0..1]", "")),
		op(diag.E4108.Def().Code, "STDLIB.md §2.2 (clamp lo > hi)", appendSite("local let zzLo: Int = 10\nlocal let zzC: Int = ", "clamp(5, zzLo, 1)", "")),
		op(diag.E4301.Def().Code, "EVALUATION.md §3.2 (cycle between values)", appendSite("", "local let zzA: Int = zzB + 1\nlocal let zzB: Int = zzA", "")),
		op(diag.E4401.Def().Code, "EVALUATION.md §12.2 (budget exhausted)", appendSite("",
			"local fn zzLoop(n: Int) -> Int {\n  var i = 0\n  while i < n { i += 1 }\n  return i\n}\n\nlocal let zzSpin: Int = zzLoop(1_000_000_000)", "")),
		op(diag.E4402.Def().Code, "EVALUATION.md §3.3 (call depth)", appendSite("",
			"local fn zzDown(n: Int) -> Int { return zzDown(n + 1) }\n\nlocal let zzDeep: Int = zzDown(0)", "")),
		op(diag.E4501.Def().Code, "STDLIB.md §2.3 (topoSort cycle)", appendSite("local let zzOrder: [String] = ",
			`topoSort(["a", "b"], next: s => if s == "a" { ["b"] } else { ["a"] })`, "")),
		op(diag.E4502.Def().Code, "STDLIB.md §4.2 (toMap key twice)", appendSite("local let zzByFirst: {String: String} = ",
			`["ab", "ac"].toMap(w => w[0..1], w => w)`, "")),
		op(diag.E4503.Def().Code, "STDLIB.md §9.5 (format spec on a String)", appendSite("local fn zzFmt(s: String) -> String {\n  return ", `"{s:.2}"`, "\n}")),
		op(diag.E5001.Def().Code, "EVALUATION.md §8.3 (package check false)", negateCheck(syntax.KwCheck)),
		op(diag.W5001.Def().Code, "EVALUATION.md §8.3 (package warn false)", appendSite("", "warn", ` [1].len() == 2 else "never"`)),
		op(diag.E5002.Def().Code, "EVALUATION.md §8.3 (fail in a check block)", appendSite("check {\n  fail(", "1", `, "zz")`+"\n}")),
		op(diag.W5002.Def().Code, "EVALUATION.md §8.3 (warn in a check block)", appendSite("check {\n  warn(", "1", `, "zz")`+"\n}")),
		op(diag.E5003.Def().Code, "EVALUATION.md §8.3 (check name twice)", checkNameTwice),
		op(diag.E6001.Def().Code, "LOCK.md §4.1 (locked entry no source has)", lockedGhost),
		op(diag.E6002.Def().Code, "LOCK.md §4.2 (retired entry back)", unretireEntry),
		op(diag.E6003.Def().Code, "LOCK.md §1 (@stable off a stable table)", stableOffTable),
		op(diag.E6004.Def().Code, "LOCK.md §6.1 (layer adds a stable entry)", layerAddsStable),
		op(diag.E6005.Def().Code, "LOCK.md §2.4 (unparsable lock line)", lockGarbage),
		op(diag.E7001.Def().Code, "WIRE.md §2 (emit out leaves the project)", emitOut(`"../../../../../zz/"`)),
		op(diag.E7002.Def().Code, "WIRE.md §6.1 (load without a type)", appendSite("local let zzL = ", `load("x.json")`, "")),
		op(diag.E7003.Def().Code, "WIRE.md §2.1 (unknown root in emit out)", emitOut(`"@nowhere/zz/"`)),
		op(diag.E7116.Def().Code, "WIRE.md §6.1 (load form for its type)", appendSite("local let zzL: Int = ", `load.dir("x/*.json")`, "")),
	}
}

// deprecatedFieldUse deprecates a field used once in its package, templates included (TYPES.md §16).
func deprecatedFieldUse(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	uses := fieldUses(tg)
	var pkg []map[string][]syntax.Node
	grouped := map[string]bool{} // also views' W1642 (VIEWMODEL.md L6)
	for _, p := range peers(tg) {
		pkg = append(pkg, fieldUses(p))
		groupedFields(p, grouped)
	}
	once := func(name string) bool {
		n := 0
		for _, u := range pkg {
			n += len(u[name])
		}
		return n == 1 && len(uses[name]) == 1 && !grouped[name]
	}
	return sitesOf(tg, func(f *syntax.FieldDecl) bool {
		return len(f.Annotations) == 0 && once(f.Name.Name) && len(fieldDecls(tg)[f.Name.Name]) == 1
	}, func(f *syntax.FieldDecl) progen.Site {
		_, e := span(tg, f)
		us, ue := span(tg, uses[f.Name.Name][0])
		return seq(1, insert(e, " @deprecated"), mark(tg, us, ue))
	})
}

// groupedFields adds to into the names tg's view groups list.
func groupedFields(tg target, into map[string]bool) {
	for _, g := range nodes[*syntax.ViewGroup](tg) {
		for _, m := range g.Members {
			if f, ok := m.(*syntax.ViewField); ok && f.Name != nil {
				into[f.Name.Name] = true
			}
		}
	}
}

// fieldUses are the names tg's file gives a value in a literal, selects or reads, by name.
func fieldUses(tg target) map[string][]syntax.Node {
	uses := map[string][]syntax.Node{}
	if tg.file == nil {
		return uses
	}
	add := func(id *syntax.Ident) {
		if id != nil {
			uses[id.Name] = append(uses[id.Name], id)
		}
	}
	syntax.Inspect(tg.file, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.FieldItem:
			add(n.Name)
		case *syntax.SelectorExpr:
			add(n.Name)
		case *syntax.IdentExpr:
			uses[n.Name] = append(uses[n.Name], n)
		}
		return true
	})
	return uses
}

// negateCheck turns the == of every package-level one-line check into !=.
func negateCheck(kw syntax.TokenKind) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) {
			return nil
		}
		var out []progen.Site
		for _, d := range tg.file.Decls {
			c, ok := d.(*syntax.CheckDecl)
			if !ok || c.Keyword != kw || c.Cond == nil {
				continue
			}
			b, ok := c.Cond.(*syntax.BinaryExpr)
			if !ok || b.Op != syntax.TokEq {
				continue
			}
			ks, ke := tokSpan(tg, c.First())
			os, oe := tokSpan(tg, b.OpTok)
			out = append(out, seq(0, mark(tg, ks, ke), replace(os, oe, "!=")))
		}
		return out
	}
}

func checkNameTwice(tg target) []progen.Site {
	return sitesOf(tg, func(c *syntax.CheckDecl) bool { return c.Name != nil && startsLine(tg, c) }, func(c *syntax.CheckDecl) progen.Site {
		s, e := span(tg, c)
		kw := tg.file.Tokens[c.First()].Kind.String()
		at := lineEnd(tg, e) + 1
		return keeping(tg, seq(1, insert(at, indent(tg, s)+kw+" "), insert(at, c.Name.Name), insert(at, ": true else \"zz\"\n")), c.Name)
	})
}

// lockLine is the region of the lock line naming key in table, and the lock's path.
func lockLine(tg target, table, key string) (*progen.Place, bool) {
	name := path.Join(path.Dir(tg.path), lockFile)
	var lock []byte
	for _, t := range *tg.all {
		if t.path == name {
			lock = t.src
		}
	}
	line := "table  " + tg.pkg + "." + table + "  " + key + "\n"
	i := strings.Index(string(lock), line)
	if i < 0 {
		return nil, false
	}
	return &progen.Place{Path: name, Region: progen.Region{Start: i, End: i}}, true
}

// stableEntries are the entries of the file's stable tables, with their table.
func stableEntries(tg target, f func(let string, e *syntax.EntryItem)) {
	for _, d := range nodes[*syntax.LetDecl](tg) {
		t, ok := d.Type.(*syntax.TableType)
		lit, isLit := d.Value.(*syntax.BraceLit)
		if !ok || !t.Stable.Valid() || !isLit {
			continue
		}
		for _, it := range lit.Items {
			if e, ok := it.(*syntax.EntryItem); ok {
				f(d.Name.Name, e)
			}
		}
	}
}

// lockedGhost adds to a lock a line for an entry its table does not have: the entry was removed
// (deleting it from the source would also change what the package's checks see).
func lockedGhost(tg target) []progen.Site {
	if path.Base(tg.path) != lockFile {
		return nil
	}
	var out []progen.Site
	seen := map[string]bool{}
	for _, line := range strings.Split(string(tg.src), "\n") {
		f := strings.Fields(line)
		if len(f) == lockFields && f[0] == "table" && !seen[f[1]] {
			seen[f[1]] = true
			out = append(out, site(insert(len(tg.src), "table  "+f[1]+"  zzgone\n")))
		}
	}
	return out
}

func unretireEntry(tg target) []progen.Site {
	var out []progen.Site
	stableEntries(tg, func(let string, e *syntax.EntryItem) {
		place, ok := lockLine(tg, let, e.Key.Name)
		if !ok {
			return
		}
		var lock []byte
		for _, t := range *tg.all {
			if t.path == place.Path {
				lock = t.src
			}
		}
		line := string(lock[place.Start : place.Start+strings.IndexByte(string(lock[place.Start:]), '\n')])
		retired := strings.Replace(string(lock), line+"\n", line+"  retired\n", 1)
		ks, ke := span(tg, e.Key)
		s := site(mark(tg, ks, ke))
		s.Add = map[string][]byte{place.Path: []byte(retired)}
		out = append(out, s)
	})
	return out
}

// stableOffTable puts @stable on an integer field of a record no stable table holds.
func stableOffTable(tg target) []progen.Site {
	elems := tableElements(tg)
	var out []progen.Site
	for _, r := range nodes[*syntax.RecordDecl](tg) {
		if elems[r.Name.Name] || r.Body == nil {
			continue
		}
		for _, it := range r.Body.Items {
			if f, ok := it.(*syntax.FieldDecl); ok && len(f.Annotations) == 0 && strings.HasPrefix(text(tg, f.Type), "Int") {
				s, e := span(tg, f)
				out = append(out, site(replace(s, e, text(tg, f)+" @stable")))
			}
		}
	}
	return out
}

func layerAddsStable(tg target) []progen.Site {
	var out []progen.Site
	stableEntries(tg, func(let string, e *syntax.EntryItem) {
		if _, ok := lockLine(tg, let, e.Key.Name); !ok {
			return
		}
		name := path.Join(path.Dir(tg.path), "zz.layer.canon")
		head := "package " + tg.pkg + "\nlayer zz\n\namend " + let + " {\n  "
		edits := []progen.Edit{insert(0, head), insert(0, "zzNew"), insert(0, ": "+text(tg, e.Value)+"\n}\n")}
		out = append(out, progen.Site{Path: name, Edits: edits, Focus: 1, Layers: []string{"zz"}})
	})
	return out
}

func lockGarbage(tg target) []progen.Site {
	if path.Base(tg.path) != lockFile {
		return nil
	}
	return []progen.Site{site(insert(len(tg.src), "zz"), insert(len(tg.src), "\n"))}
}

// emitOut replaces the out: option of every emit of the file.
func emitOut(path string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		var out []progen.Site
		for _, d := range nodes[*syntax.EmitDecl](tg) {
			if d.Target.Name == "ts" {
				continue
			}
			for _, it := range d.Options.Items {
				if f, ok := it.(*syntax.FieldItem); ok && f.Name.Name == "out" {
					s, e := span(tg, f.Value)
					out = append(out, site(replace(s, e, path)))
				}
			}
		}
		return out
	}
}
