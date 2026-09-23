package stock

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// resolveTool asks the pinned toolchain module for name's built binary path.
func resolveTool(toolchain, name string) (string, error) {
	if toolchain == "" {
		return "", errNoToolchain
	}
	//nolint:gosec // name is one of our own const tool names, never external input
	cmd := exec.CommandContext(context.Background(), goCmd, "tool", "-n", name)
	cmd.Dir = toolchain
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", name, err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("resolve %s: %w", name, errEmptyToolPath)
	}
	return path, nil
}

// runTool runs bin in dir and returns stdout; stderr is folded into the error.
func runTool(bin, dir string, args ...string) ([]byte, error) {
	//nolint:gosec // bin is resolveTool's own resolved path, never external input
	cmd := exec.CommandContext(context.Background(), bin, args...)
	cmd.Dir = dir
	cmd.Env = lane.ToolEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
