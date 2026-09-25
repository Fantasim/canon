package progen_test

import (
	"fmt"
	"strings"
)

// smokeMark starts every failure the smoke test prints, then its shape: goFailSig reads it.
const smokeMark = "progen-smoke "

// kindNames and wrapNames name a field's shape in a smoke failure, and so in its signature.
var (
	kindNames = map[tyKind]string{tyInt: "Int", tyString: "String", tyBool: "Bool", tyFloat: "Float", tyRange: "Range", tyEnum: "enum"}
	wrapNames = map[wrapKind]string{wrapNone: "plain", wrapOptional: "optional", wrapList: "list", wrapMap: "map"}
)

// renderSmoke is the Go test of the generated getters and fn (IMPLEMENTATION-PLAN.md §7.7 item 3).
func renderSmoke(tc typedCase) []byte {
	var b strings.Builder
	b.WriteString("package smoke\n\nimport (\n")
	for _, imp := range smokeImports(tc.roots) {
		fmt.Fprintf(&b, "\t%q\n", imp)
	}
	fmt.Fprintf(&b, "\t%s %q\n)\n\n", typedPackage, typedGoImport)
	b.WriteString("func TestSmoke(t *testing.T) {\n")
	for _, root := range tc.roots {
		renderRootBlock(&b, root)
	}
	for _, stmt := range probeStmts(tc.fn) {
		fmt.Fprintf(&b, "\t%s\n", stmt)
	}
	b.WriteString("}\n")
	b.WriteString(probeHelper)
	return []byte(b.String())
}

// renderRootBlock writes one root's field assertions, in its own braces so every root's loop and
// scratch variables (fieldStmt's got<Field>) stay in their own scope.
func renderRootBlock(b *strings.Builder, root typedRoot) {
	fmt.Fprintf(b, "\t{\n\t\tr := %s.Get%s()\n", typedPackage, goName(root.name))
	for _, f := range root.rec.fields {
		for _, stmt := range fieldStmt("r", f, root.vals[f.name]) {
			fmt.Fprintf(b, "\t\t%s\n", stmt)
		}
	}
	b.WriteString("\t}\n")
}

// smokeImports are the standard library packages the smoke test needs: those of the probe helper
// and strconv always, slices and maps only when a field is a list or a map (an unused import is
// a compile error, which this suite must tell apart from a generator bug).
func smokeImports(roots []typedRoot) []string {
	imports := []string{"bytes", "encoding/json", "os", "path/filepath", "strconv", "testing"}
	var sl, mp bool
	for _, root := range roots {
		for _, f := range root.rec.fields {
			sl = sl || f.ft.wrap == wrapList
			mp = mp || f.ft.wrap == wrapMap
		}
	}
	if sl {
		imports = append(imports, "slices")
	}
	if mp {
		imports = append(imports, "maps")
	}
	return imports
}

// fieldStmt is the Go statements asserting field f of r (a Go expression) equals val, the value
// genValue built the field's literal from.
func fieldStmt(r string, f fieldSpec, val any) []string {
	call := r + "." + goName(f.name) + "()"
	mark := smokeMark + kindNames[f.ft.kind] + "/" + wrapNames[f.ft.wrap] + " " + f.name + ": "
	switch f.ft.wrap {
	case wrapOptional:
		return optionalStmt(call, mark, f.ft.kind, val)
	case wrapList:
		return listStmt(call, mark, f, val)
	case wrapMap:
		return mapStmt(call, mark, f, val)
	default:
		return []string{fmt.Sprintf("if got := %s; got != %q { t.Errorf(%q, got) }", convExpr(f.ft.kind, call), wantText(f.ft.kind, val), mark+"got %v")}
	}
}

// optionalStmt asserts a field?'s (value, ok) pair: ok matches val's presence, and present its text.
func optionalStmt(call, mark string, kind tyKind, val any) []string {
	wantOK, want := val != nil, ""
	if wantOK {
		want = wantText(kind, val)
	}
	cond := fmt.Sprintf("ok != %t || (ok && %s != %q)", wantOK, convExpr(kind, "v"), want)
	return []string{fmt.Sprintf("if v, ok := %s; %s { t.Errorf(%q, ok, v) }", call, cond, mark+"ok=%v v=%v")}
}
