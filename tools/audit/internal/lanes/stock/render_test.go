package stock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

func TestRenderShippedConfig(t *testing.T) {
	limits, err := threshold.Load(filepath.Join("..", "..", "..", threshold.FileName))
	if err != nil {
		t.Fatal(err)
	}
	path, err := renderConfig(filepath.Join("..", "..", "..", "toolchain"), limits)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(path) }()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"min-complexity: " + strconv.Itoa(limits.FnComplexity) + "\n",
		"threshold: " + strconv.Itoa(limits.DupTokens) + "\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered config lacks %q", want)
		}
	}
	if again, err := limits.Expand(text); err != nil || again != text {
		t.Errorf("rendered config still holds a placeholder (err %v)", err)
	}
}

func TestRenderRefusesUnknownKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, golangciConfig), []byte("min-complexity: {nope}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := renderConfig(dir, threshold.Set{}); !errors.Is(err, errConfig) {
		t.Fatalf("renderConfig error = %v, want errConfig", err)
	}
	if _, err := renderConfig(filepath.Join(dir, "missing"), threshold.Set{}); !errors.Is(err, errConfig) {
		t.Fatalf("renderConfig of a missing template: error = %v, want errConfig", err)
	}
}
