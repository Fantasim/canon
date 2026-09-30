package canon

import (
	"maps"
	"path/filepath"
	"runtime"
	"testing"
)

// API.md §2.1, §2.2 (log M4 B12-r): Options.Roots become FS names, relative ones kept relative.
func TestFromAPIRoots(t *testing.T) {
	if fromAPIRoots(nil, "/law") != nil {
		t.Error("no roots became some")
	}
	dir, err := absolute("/law")
	if err != nil {
		t.Fatal(err)
	}
	vol := filepath.VolumeName(dir)
	in := map[string]string{"out": "out/go", "abs": "/x/../y"}
	want := map[string]string{"out": "out/go", "abs": vol + "/y"}
	if runtime.GOOS == "windows" {
		in["win"], want["win"] = `gen\ts`, "gen/ts"
	}
	got := fromAPIRoots(in, dir)
	if !maps.Equal(got, want) {
		t.Errorf("roots %v, want %v", got, want)
	}
	got["out"] = "changed"
	if in["out"] != "out/go" {
		t.Error("the caller's roots were changed")
	}
}
