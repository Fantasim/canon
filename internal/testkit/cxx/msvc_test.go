package cxx_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// TestMSVCCompileArgs proves MSVCCompileArgs' argv (CODEGEN.md §9).
func TestMSVCCompileArgs(t *testing.T) {
	tests := []struct {
		name string
		mode []string
		tail []string
	}{
		{name: "exceptions on", mode: cxx.MSVCModes[0], tail: []string{"/EHsc"}},
		{name: "exceptions off", mode: cxx.MSVCModes[1], tail: []string{"/D_HAS_EXCEPTIONS=0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := append(append([]string(nil), cxx.MSVCFlags...), tt.tail...)
			want = append(want, "/Idir", "/Iinc", "/c", "/Foobj.obj", "src.cpp")
			got := cxx.MSVCCompileArgs(tt.mode, "dir", "inc", "obj.obj", "src.cpp")
			if !slices.Equal(got, want) {
				t.Errorf("MSVCCompileArgs(%v, ...) = %v, want %v", tt.mode, got, want)
			}
		})
	}
}

// TestMSVCLinkArgs proves MSVCLinkArgs' argv (CODEGEN.md §9).
func TestMSVCLinkArgs(t *testing.T) {
	want := append(append([]string(nil), cxx.MSVCFlags...), "/D_HAS_EXCEPTIONS=0")
	want = append(want, "/Febin.exe", "a.obj", "b.obj")
	got := cxx.MSVCLinkArgs(cxx.MSVCModes[1], "bin.exe", []string{"a.obj", "b.obj"})
	if !slices.Equal(got, want) {
		t.Errorf("MSVCLinkArgs(...) = %v, want %v", got, want)
	}
}

// TestDetectMSVCNotWindows proves DetectMSVC is false off Windows (IMPLEMENTATION-PLAN.md §7.8).
func TestDetectMSVCNotWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this case only proves the non-Windows branch")
	}
	if p, ok := cxx.DetectMSVC(t); ok {
		t.Errorf("DetectMSVC() = %q, true, want false off Windows", p)
	}
}
