package build

import (
	"runtime/debug"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// DECISIONS 354: the compiler version is the link-time one, else the module version at a release tag, else dev.
func TestReleaseOf(t *testing.T) {
	main := func(version string) debug.BuildInfo {
		return debug.BuildInfo{Main: debug.Module{Path: ModulePath, Version: version}}
	}
	dep := func(version string, replaced bool) debug.BuildInfo {
		m := &debug.Module{Path: ModulePath, Version: version}
		if replaced {
			m.Replace = &debug.Module{Path: "../canon"}
		}
		return debug.BuildInfo{Main: debug.Module{Path: "example.com/studio"}, Deps: []*debug.Module{m}}
	}
	cases := []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"go install at a tag", main("v0.1.5"), "0.1.5"},
		{"a pre-release tag", main("v0.2.0-rc.1"), "0.2.0-rc.1"},
		{"a source build", main("(devel)"), ""},
		{"a pseudo-version", main("v0.1.6-0.20260923120000-abcdef123456"), ""},
		{"a dirty build", main("v0.1.5+dirty"), ""},
		{"an unversioned main", main(""), ""},
		{"a tagged dependency", dep("v0.1.5", false), "0.1.5"},
		{"a replaced dependency", dep("v0.1.5", true), ""},
		{"another main module", debug.BuildInfo{Main: debug.Module{Path: "x", Version: "v1.0.0"}}, ""},
	}
	for _, c := range cases {
		if got := releaseOf(&c.info); got != c.want {
			t.Errorf("%s: releaseOf = %q, want %q", c.name, got, c.want)
		}
	}
}

// DECISIONS 354: the link-time value wins; an unstamped test binary is dev, never 0.1.0.
func TestCompiler(t *testing.T) {
	if got := Compiler(); got != project.DevVersion {
		t.Errorf("Compiler() = %q, want %q", got, project.DevVersion)
	}
	defer func(old string) { CompilerVersion = old }(CompilerVersion)
	CompilerVersion = "0.1.5"
	if got := Compiler(); got != "0.1.5" {
		t.Errorf("Compiler() = %q after a link-time value", got)
	}
}
