package winpaths_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// WIRE.md §2.1, API.md §2.2: the model returns what Go's filepath.VolumeName returns on Windows.
func TestVolumeName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"C:/x", "C:"},
		{`c:\x`, "c:"},
		{"C:", "C:"},
		{"/law/x", ""},
		{"law/x", ""},
		{"", ""},
		{"//server/share/x", `\\server\share`},
		{`\\server\share\x`, `\\server\share`},
		{"//server/share", `\\server\share`},
		{"//server/share/", `\\server\share`},
		{"//wsl$/Ubuntu/home", `\\wsl$\Ubuntu`},
		{"//server", `\\server`},
		{"//server/", `\\server\`},
		{"//", `\\`},
		{"//?/C:/x", `\\?\C:`},
		{`\\.\COM1`, `\\.\COM1`},
		{"//./UNC/host/share/x", `\\.\UNC\host\share`},
		{`\??\C:\x`, `\??\C:`},
		{`\\.`, `\\.`},
	}
	for _, c := range cases {
		if got := winpaths.VolumeName(c.name); got != c.want {
			t.Errorf("VolumeName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
