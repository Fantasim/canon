package progen_test

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on the emit modes of the data a package loads (CODEGEN.md §2.8, §5.11).
func modesOperators() []operator {
	return []operator{
		op(diag.E8018.Def().Code, "CODEGEN.md §2.8 (data mode decodes a type of a baked package)", bakedFieldType),
		op(diag.E8201.Def().Code, "CODEGEN.md §5.11 (@reload on a legacy struct in fields mode)", reloadLegacy),
	}
}

// bakedPackage is a package emitted in baked mode, whose record a data-mode emit then decodes.
const bakedPackage = "zzbaked/zzbaked.canon"

const bakedSource = `/// A package emitted in baked mode.
package zzbaked

/// A point.
record ZzPoint {
  /// Across.
  x: Int
}

emit go { out: "@sovcommon/zzbaked", package: "zzbaked", mode: baked }
emit cpp { out: "@source/Generated/zzbaked", namespace: "zz", mode: types }
`

// bakedFieldType gives the record a data-mode go emit loads a field typed in a new baked
// package; the finding is at the emit's target (decision log, "E8018 location").
func bakedFieldType(tg target) []progen.Site {
	l, ok := loadedRecord(tg)
	last := lastField(l)
	if !ok || !dataMode(tg) || last == nil || !endsLine(tg, last) {
		return nil
	}
	_, e := span(tg, last)
	ind := indent(tg, e)
	field := insert(lineEnd(tg, e)+1, ind+"/// Zz.\n"+ind+`zzAt: ZzPoint? @json("zzAt")`+"\n")
	imp := insert(importAt(tg), "import zzbaked { ZzPoint }\n")
	var out []progen.Site
	for _, d := range emitsOf(tg, "go") {
		if optionText(tg, d, "mode") == "data" {
			ts, te := span(tg, d.Target)
			st := seq(2, imp, field, mark(tg, ts, te))
			st.Add = map[string][]byte{bakedPackage: []byte(bakedSource)}
			out = append(out, st)
		}
	}
	return out
}

// loadedRecord is the record of a load.dir of tg declared in tg itself.
func loadedRecord(tg target) (loaded, bool) {
	for _, d := range nodes[*syntax.LetDecl](tg) {
		m := elemOf.FindStringSubmatch(textOr(tg, d.Type))
		if m == nil || !slices.ContainsFunc(dirLoads(tg), func(l dirLoad) bool { return l.call == d.Value }) {
			continue
		}
		if r, ok := recordOf(tg, m[1]+m[2]); ok && r.src.path == tg.path {
			return r, true
		}
	}
	return loaded{}, false
}

// importAt is where a new import line goes: after the last import, else after the package clause.
func importAt(tg target) int {
	if n := len(tg.file.Imports); n > 0 {
		_, e := span(tg, tg.file.Imports[n-1])
		return lineEnd(tg, e) + 1
	}
	_, e := span(tg, tg.file.Package)
	return lineEnd(tg, e) + 1
}

// reloadLegacy maps onto a legacy struct in fields mode the record of each @reload value of a
// file with a data-mode cpp emit; the finding is at the value's name.
func reloadLegacy(tg target) []progen.Site {
	if !cppData(tg) {
		return nil
	}
	var out []progen.Site
	for _, d := range nodes[*syntax.LetDecl](tg) {
		m := elemOf.FindStringSubmatch(textOr(tg, d.Type))
		if !annotated(d.Annotations, "reload") || m == nil {
			continue
		}
		r, ok := recordOf(tg, m[1]+m[2])
		if !ok || r.src.path != tg.path || annotated(r.record.Annotations, "cpp") {
			continue
		}
		bs, _ := span(tg, r.record.Body)
		ns, ne := span(tg, d.Name)
		legacy := insert(bs, `@cpp(struct: "ZzProp", header: "ZzProp.h", access: fields) `)
		out = append(out, seq(1, legacy, mark(tg, ns, ne)))
	}
	return out
}

func cppData(tg target) bool {
	for _, d := range emitsOf(tg, "cpp") {
		if optionText(tg, d, "mode") == "data" {
			return true
		}
	}
	return false
}

func annotated(as []*syntax.Annotation, name string) bool {
	for _, a := range as {
		if a.Name != nil && a.Name.Name == name {
			return true
		}
	}
	return false
}

// textOr is the source of n, "" for none.
func textOr(tg target, n syntax.Node) string {
	if n == nil {
		return ""
	}
	return text(tg, n)
}
