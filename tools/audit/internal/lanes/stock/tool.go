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

// runTool runs bin in dir under ctx (a caller deadline included: a stuck lock wait is bounded
// only by killing the process) and returns stdout, folding stderr into the error. extraEnv
// wins over the caller's environment: exec keeps the last value of a duplicated key.
func runTool(ctx context.Context, bin, dir string, extraEnv []string, args ...string) ([]byte, error) {
	//nolint:gosec // bin is resolveTool's own resolved path, never external input
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(lane.ToolEnv(), extraEnv...)
	cmd.WaitDelay = toolWaitDelay
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("%s: %w %s", filepath.Base(bin), ctxErr, msgKilled)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
