package progen_test

import (
	"fmt"
	"maps"
	"slices"
)

// listStmt asserts a `[T]` field's elements as canonical text, CODEGEN.md §5.1's rt.List.All().
func listStmt(call, mark string, f fieldSpec, val any) []string {
	items := val.([]any)
	want := make([]string, len(items))
	for i, v := range items {
		want[i] = wantText(f.ft.kind, v)
	}
	got := "got" + goName(f.name)
	return []string{
		fmt.Sprintf("var %s []string", got),
		fmt.Sprintf("for e := range %s.All() { %s = append(%s, %s) }", call, got, got, convExpr(f.ft.kind, "e")),
		fmt.Sprintf("if want := %s; !slices.Equal(%s, want) { t.Errorf(%q, %s) }", goStringSlice(want), got, mark+"got %v", got),
	}
}

// mapStmt asserts a `{String: T}` field's entries as canonical text, CODEGEN.md §5.1's rt.Map.All().
func mapStmt(call, mark string, f fieldSpec, val any) []string {
	vm := val.(map[string]any)
	got := "got" + goName(f.name)
	want := goStringMap(slices.Sorted(maps.Keys(vm)), func(k string) string { return wantText(f.ft.kind, vm[k]) })
	return []string{
		fmt.Sprintf("%s := map[string]string{}", got),
		fmt.Sprintf("for k, v := range %s.All() { %s[k] = %s }", call, got, convExpr(f.ft.kind, "v")),
		fmt.Sprintf("if want := %s; !maps.Equal(%s, want) { t.Errorf(%q, %s) }", want, got, mark+"got %v", got),
	}
}
