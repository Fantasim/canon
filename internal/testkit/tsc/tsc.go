package tsc

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/tsc/config"
)

// Compiler is one TypeScript compiler: a name for messages and the script node runs.
type Compiler struct {
	Name, Script string
}

// Toolchain is node, the compilers and the Node typings, found under tools/tsc.
type Toolchain struct {
	Node      string
	Compilers []Compiler
	Typings   string
}

// Find locates the toolchain, or gates the test: it is skipped, or fails when config.RequireTS is set.
func Find(t *testing.T) Toolchain {
	t.Helper()
	node, err := exec.LookPath(nodeName)
	if err != nil {
		gate(t, noNodeMsg)
	}
	root, ok := findTools()
	if !ok {
		gate(t, noTscMsg)
	}
	tc := Toolchain{Node: node, Typings: filepath.Join(root, modulesDir, typingsDir)}
	for _, name := range compilers {
		script := filepath.Join(root, modulesDir, name, filepath.FromSlash(compilerPath))
		if _, err := os.Stat(script); err == nil {
			tc.Compilers = append(tc.Compilers, Compiler{Name: name, Script: script})
		}
	}
	if len(tc.Compilers) == 0 {
		gate(t, noTscMsg)
	}
	if config.RequireTS() && len(tc.Compilers) < len(compilers) {
		t.Fatal(oneTscMsg)
	}
	return tc
}

// gate skips the test on msg, or fails it when config.RequireTS is set.
func gate(t *testing.T, msg string) {
	t.Helper()
	if config.RequireTS() {
		t.Fatal(msg)
	}
	t.Skip(msg)
}

// findTools is the absolute tools/tsc directory above the working directory, if it has been installed.
func findTools() (string, bool) {
	up := ""
	for i := 0; i <= maxParentDirs; i++ {
		dir := filepath.Join(up, filepath.FromSlash(toolsDir))
		if _, err := os.Stat(filepath.Join(dir, modulesDir)); err == nil {
			abs, err := filepath.Abs(dir)
			return abs, err == nil
		}
		up = filepath.Join(up, "..")
	}
	return "", false
}

// Args are the compiler's arguments to check files under --strict (CODEGEN.md §9): ES2020, ESM, no unused locals, the Node typings.
func Args(typings string, files ...string) []string {
	args := []string{
		flagStrict, flagNoUnused, flagTarget, valueTarget, flagModule, valueModule, flagResolution, valueResolution,
		flagTypes, valueNodeTypes, flagTypeRoots, typings,
	}
	return append(args, files...)
}
