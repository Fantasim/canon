package build_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	perPackageProject = "project acme {\n  canon: \"0.1\"\n  budget: 200\n}\n"
	budgetLib         = `/// A.
package a

/// Loops n times.
fn loop(n: Int) -> Int {
  var i = 0
  while i < n { i += 1 }
  return i
}

/// Past a's budget.
let big: Int = loop(1000)

/// Left unevaluated once a's budget is spent.
let after: Int = 2

test "cheap" {
  expect loop(2) == 2
}
`
	budgetUser = `/// B.
package b

import a

test "reads a's big" {
  expect a.big == 1000
}

test "runs on b's budget" {
  expect a.loop(25) == 25
}
`
	budgetLater = `/// C.
package c

import a

test "reads a's big again" {
  expect a.big == 1000
}

test "reads what a's budget left" {
  expect a.after == 2
}
`
)

// EVALUATION.md §10.1, §12.2, API.md R6: tests spend their package's budget; a's spent budget fails its readers' expects, as their cause.
func TestTestBudgetPerPackage(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(perPackageProject), "p/a/a.canon": file(budgetLib), "p/b/b.canon": file(budgetUser), "p/c/c.canon": file(budgetLater)}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Test(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"cheap": "", "reads a's big": "a.big", "runs on b's budget": "", "reads a's big again": "a.big", "reads what a's budget left": "a.after"}
	if len(res.Tests) != len(want) {
		t.Fatalf("tests %+v, want %d", res.Tests, len(want))
	}
	for _, tc := range res.Tests {
		if tc.Passed != (want[tc.Name] == "") {
			t.Errorf("%s.%s: passed %t, want %t: %+v", tc.Package, tc.Name, tc.Passed, want[tc.Name] == "", tc.Failures)
		}
		if tc.Passed {
			continue
		}
		if len(tc.Failures) != 1 || tc.Failures[0].Poisoned != want[tc.Name] || len(tc.Failures[0].Findings) != 0 {
			t.Errorf("%s.%s: failures %+v, want the expect reading %s alone, no stopping error", tc.Package, tc.Name, tc.Failures, want[tc.Name])
			continue
		}
		if c := tc.Failures[0].Cause; len(c) != 1 || c[0].Code != diag.E4401.Def().Code || c[0].Package != "a" {
			t.Errorf("%s.%s: cause %+v, want a's spent budget", tc.Package, tc.Name, c)
		}
	}
}
