package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
)

// DECISIONS 343: an output lies under a consumer root by location, the closest consumer root
// holding its directory, whichever root its out names; a root that is no consumer root is none.
func TestConsumerRoot(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	admin, inner := rootAt("admin", "../admin", 10), rootAt("inner", "../admin/x/inner", 20)
	admin.Consumer, inner.Consumer = true, true
	p.Roots = []project.Root{admin, inner, rootAt("sub", "../admin/sub", 30), rootAt("web", "../web", 40)}
	for _, c := range []struct{ dir, want string }{
		{"../admin", "admin"},
		{"../admin/go/a", "admin"},
		{"../admin/sub/notes", "admin"}, // @sub lies inside @admin
		{"../admin/x/inner/y", "inner"}, // the closest of two
		{"../web/a", ""},
		{"../adminx/a", ""}, // a string prefix of admin, not a child of it
		{"out", ""},
	} {
		if got := check.ConsumerRoot(p, c.dir); got != c.want {
			t.Errorf("ConsumerRoot(%q) = %q, want %q", c.dir, got, c.want)
		}
	}
}
