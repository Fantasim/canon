//go:build knownbug

package canon_test

import (
	"context"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// API.md F1: a finding of a `let` initializer has that let as its Path, which Value resolves; KNOWN BUG, Path is "".
func TestFindingPathOfLetInitializer(t *testing.T) {
	findings, p := checkProject(t)
	code := diag.E4102.Def().Code
	for _, f := range findings {
		if diag.Code(f.Code) != code {
			continue
		}
		if f.Path != "zero" {
			t.Fatalf("Path = %q, want %q", f.Path, "zero")
		}
		_, err := p.Value(context.Background(), f.Package+":"+f.Path)
		if !errors.Is(err, canon.ErrNoValue) {
			t.Errorf("Value(%s:%s): %v, want ErrNoValue", f.Package, f.Path, err)
		}
		return
	}
	t.Fatalf("no %s finding", code)
}
