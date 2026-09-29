package cxx

import (
	"os/exec"
	"runtime"
	"testing"
)

// DetectMSVC finds cl.exe on PATH; it never succeeds off Windows (IMPLEMENTATION-PLAN.md §7.8).
func DetectMSVC(t *testing.T) (string, bool) {
	t.Helper()
	if runtime.GOOS != osWindows {
		return "", false
	}
	p, err := exec.LookPath(clExe)
	return p, err == nil
}

// MSVCCompileArgs is the cl.exe argv that compiles src into obj, in mode (one of MSVCModes),
// with dir and include on cl.exe's search path: the MSVC analog of g++/clang++'s
// "-I dir -I include -c -o obj src".
func MSVCCompileArgs(mode []string, dir, include, obj, src string) []string {
	args := append(append([]string(nil), MSVCFlags...), mode...)
	for _, d := range []string{dir, include} {
		args = append(args, msvcIncludeFlag+d)
	}
	return append(args, msvcCompileFlag, msvcObjFlag+obj, src)
}

// MSVCLinkArgs is the cl.exe argv that links objs into bin, in mode.
func MSVCLinkArgs(mode []string, bin string, objs []string) []string {
	args := append(append([]string(nil), MSVCFlags...), mode...)
	args = append(args, msvcExeFlag+bin)
	return append(args, objs...)
}
