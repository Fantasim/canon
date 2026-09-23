package stock

import (
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

const filterSrc = `package api

import (
	"context"
	"database/sql"
	"net/http"
)

func dbCtx(r *http.Request) context.Context { return r.Context() }

func handle(w http.ResponseWriter, r *http.Request) {
	_ = dbCtx(r)
}

func worker() {
	_ = dbCtx(nil)
	load()
}

func other() {
	load()
}

func load() {}

func scan(rows *sql.Rows) (int, error) {
	var n int
	return n, rows.Scan(&n)
}

func plain(rows *sql.Rows) error {
	return rows.Scan()
}
`

func filterTree(t *testing.T) *lane.Context {
	t.Helper()
	dir := t.TempDir()
	writeGoFile(t, dir, "internal/api/api.go", filterSrc)
	writeGoFile(t, dir, "internal/e2e/client.go", "package e2e\n\nfunc run() error { return nil }\n")
	tree, errs := gosrc.Parse(dir, []string{"internal/api/api.go", "internal/e2e/client.go"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	ctx := allOn()
	ctx.Go = tree
	return ctx
}

func issue(linter, file string, line int, text string) golangciIssue {
	return golangciIssue{FromLinter: linter, Text: text, Pos: golangciPos{Filename: file, Line: line}}
}

func TestStockFilters(t *testing.T) {
	const api = "internal/api/api.go"
	passLoad := "Function `other->load` should pass the context parameter"
	issues := []golangciIssue{
		issue(linterContextcheck, api, 12, "Function `dbCtx` should pass the context parameter"),
		issue(linterContextcheck, api, 16, "Function `dbCtx` should pass the context parameter"),
		issue(linterContextcheck, api, 17, "Function `load` should pass the context parameter"),
		issue(linterContextcheck, api, 21, passLoad),
		issue(linterWrapcheck, api, 28, "error returned from external package is unwrapped"),
		issue(linterWrapcheck, api, 32, "error returned from external package is unwrapped"),
		issue(linterGosec, "internal/e2e/client.go", 3, "G104: errors unhandled"),
	}
	fs := golangciFindings(filterTree(t), issues)
	var got []finding.Finding
	for _, f := range fs {
		got = append(got, finding.Finding{Rule: f.Rule, Line: f.Line, Value: f.Value})
	}
	want := []finding.Finding{
		{Rule: ruleCtxFirst, Line: 17, Value: 2},
		{Rule: ruleErrUnwrapped, Line: 32},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if d := fs[0].Detail; d != "contextcheck: Function `load` should pass the context parameter" {
		t.Errorf("ctx-first detail = %q, want the final callee only", d)
	}
}
