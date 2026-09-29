package canon

import (
	"runtime/debug"
	"testing"
)

// API.md T3: Commit is the compiler's revision, not the embedding program's; "" when unknown.
func TestCommitOf(t *testing.T) {
	stamped := func(modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"}, {Key: "vcs.modified", Value: modified}}
	}
	dep := func(version string, replaced bool) []*debug.Module {
		m := &debug.Module{Path: modulePath, Version: version}
		if replaced {
			m.Replace = &debug.Module{Path: "../canonlang"}
		}
		return []*debug.Module{{Path: "golang.org/x/tools", Version: "v0.1.0"}, m}
	}
	cases := []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"canon built clean", debug.BuildInfo{Main: debug.Module{Path: modulePath}, Settings: stamped("false")}, "0123456789abcdef0123456789abcdef01234567"},
		{"canon built from a modified tree", debug.BuildInfo{Main: debug.Module{Path: modulePath}, Settings: stamped("true")}, ""},
		{"canon built without VCS", debug.BuildInfo{Main: debug.Module{Path: modulePath}}, ""},
		{"embedder's own revision", debug.BuildInfo{Main: debug.Module{Path: "example.com/studio"}, Settings: stamped("false")}, ""},
		{"pseudo-version", debug.BuildInfo{Main: debug.Module{Path: "example.com/studio"}, Deps: dep("v0.0.0-20260923120000-abcdef123456", false)}, "abcdef123456"},
		{"pseudo-version after a tag", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.1.1-0.20260923120000-abcdef123456", false)}, "abcdef123456"},
		{"pseudo-version after a pre-release", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.2.0-rc.1.0.20260923120000-abcdef123456", false)}, "abcdef123456"},
		{"tag", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.1.0", false)}, ""},
		{"pre-release", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.1.0-rc.1", false)}, ""},
		{"upper-case revision", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.0.0-20260923120000-ABCDEF123456", false)}, ""},
		{"short stamp", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0-2026-abcdef123456", false)}, ""},
		{"replaced", debug.BuildInfo{Main: debug.Module{Path: "x"}, Deps: dep("v0.0.0-20260923120000-abcdef123456", true)}, ""},
		{"not a dependency", debug.BuildInfo{Main: debug.Module{Path: "x"}}, ""},
	}
	for _, c := range cases {
		if got := commitOf(&c.info); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
