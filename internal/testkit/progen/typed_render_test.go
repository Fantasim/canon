package progen_test

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// typedRoot is one public root value: a record instance, its literal text and its field values (IMPLEMENTATION-PLAN.md §7.7 item 3).
type typedRoot struct {
	name string
	rec  recordDef
	text string
	vals map[string]any
}

// genTypedRoots is one root per record of mo, so every generated field kind and wrap is a public
// value some `emit go`/`emit json` output holds; the first omits every defaulted field.
func genTypedRoots(r *progen.Rand, mo *typedModel) []typedRoot {
	roots := make([]typedRoot, 0, len(mo.records))
	for i, rec := range mo.records {
		text, vals := genRecordValue(r, mo, rec, i == 0)
		roots = append(roots, typedRoot{name: fmt.Sprintf("v%d", i), rec: rec, text: text, vals: vals})
	}
	return roots
}

// renderTypedSource is the Canon source of tc, every type and field documented (W1002).
func renderTypedSource(tc typedCase) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n", typedPackage)
	fmt.Fprintf(&b, "const %s = 0\nconst %s = \"\"\n", zzIntBase, zzStrBase)
	for _, e := range tc.mo.enums {
		fmt.Fprintf(&b, "/// %s.\nenum %s {\n", e.name, e.name)
		for _, m := range e.members {
			fmt.Fprintf(&b, "  %s\n", m)
		}
		b.WriteString("}\n")
	}
	for _, rec := range tc.mo.records {
		renderRecord(&b, tc.mo, rec)
	}
	names := renderTypedRoots(&b, tc.roots)
	renderTable(&b, tc.table)
	renderRef(&b, tc.ref)
	names = append(names, tc.table.name, tc.ref.name)
	names = append(names, renderFn(&b, tc.fn)...)
	fmt.Fprintf(&b, "emit go { out: \"@%s/go/\", package: %q, mode: baked }\n", typedOutRoot, typedPackage)
	fmt.Fprintf(&b, "emit json { out: \"@%s/data/\", values: [%s] }\n", typedOutRoot, strings.Join(names, ", "))
	return []byte(b.String())
}

// renderRecord writes one record declaration.
func renderRecord(b *strings.Builder, mo *typedModel, rec recordDef) {
	fmt.Fprintf(b, "/// %s.\nrecord %s {\n", rec.name, rec.name)
	for _, f := range rec.fields {
		def := ""
		if f.def != nil {
			def = " = " + f.def.expr
		}
		fmt.Fprintf(b, "  /// %s.\n  %s: %s%s\n", f.name, f.name, typeText(mo, f.ft), def)
	}
	b.WriteString("}\n")
}

// renderTypedRoots writes roots' `let` declarations and returns their names, in order.
func renderTypedRoots(b *strings.Builder, roots []typedRoot) []string {
	names := make([]string, 0, len(roots))
	for _, root := range roots {
		fmt.Fprintf(b, "let %s: %s = %s\n", root.name, root.rec.name, root.text)
		names = append(names, root.name)
	}
	return names
}

// goName is a field, root or fn's Go name (CODEGEN.md §3.2): its lowerCamel name, capitalized.
func goName(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// convExpr is the Go expression converting expr, a kind-typed getter result, to canonical text.
func convExpr(kind tyKind, expr string) string {
	switch kind {
	case tyString:
		return expr
	case tyEnum:
		return expr + ".String()"
	case tyBool:
		return "strconv.FormatBool(" + expr + ")"
	case tyFloat:
		return "strconv.FormatFloat(" + expr + ", 'g', -1, 64)"
	default: // tyInt, tyRange: CODEGEN.md §5.1's int64
		return "strconv.FormatInt(" + expr + ", 10)"
	}
}

// wantText is val's canonical text, in the same form convExpr's generated code produces.
func wantText(kind tyKind, val any) string {
	switch kind {
	case tyString, tyEnum:
		return val.(string)
	case tyBool:
		return strconv.FormatBool(val.(bool))
	case tyFloat:
		return strconv.FormatFloat(val.(float64), 'g', -1, 64)
	default:
		return strconv.FormatInt(int64(val.(float64)), 10)
	}
}

// goStringSlice is a Go source `[]string{...}` literal of ss.
func goStringSlice(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = strconv.Quote(s)
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

// goStringMap is a Go source `map[string]string{...}` literal of keys, each mapped through text.
func goStringMap(keys []string, text func(string) string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = strconv.Quote(k) + ": " + strconv.Quote(text(k))
	}
	return "map[string]string{" + strings.Join(parts, ", ") + "}"
}
