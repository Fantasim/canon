package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// unifiedDiff: CLI.md §3.6, `--diff` prints the changes as a unified diff, through go-udiff (IMPLEMENTATION-PLAN §11).
func TestUnifiedDiff(t *testing.T) {
	long := func(n int, from string) string {
		var sb strings.Builder
		for i := 1; i <= n; i++ {
			sb.WriteString(from + strconv.Itoa(i) + "\n")
		}
		return sb.String()
	}
	tests := []struct{ name, old, updated, want string }{
		{"equal", "a\nb\n", "a\nb\n", ""},
		{"change", "a\nb\nc\n", "a\nB\nc\n", "--- f\n+++ f\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n"},
		{"from empty", "", "a\n", "--- f\n+++ f\n@@ -0,0 +1 @@\n+a\n"},
		{"to empty", "a\nb\n", "", "--- f\n+++ f\n@@ -1,2 +0,0 @@\n-a\n-b\n"},
		{"final line break", "a", "a\n", "--- f\n+++ f\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+a\n"},
		{"crlf", "a\r\n", "a\n", "--- f\n+++ f\n@@ -1 +1 @@\n-a\r\n+a\n"},
		{
			"two hunks", long(20, "l") + "old\n" + long(20, "m") + "old\n" + long(3, "n"), long(20, "l") + "new\n" + long(20, "m") + "new\n" + long(3, "n"),
			"--- f\n+++ f\n@@ -18,7 +18,7 @@\n l18\n l19\n l20\n-old\n+new\n m1\n m2\n m3\n" +
				"@@ -39,7 +39,7 @@\n m18\n m19\n m20\n-old\n+new\n n1\n n2\n n3\n",
		},
		{
			"hunks that touch merge", "1\n2\n3\n4\n5\n6\n7\n8\n9\n", "1\nX\n3\n4\n5\n6\n7\nY\n9\n",
			"--- f\n+++ f\n@@ -1,9 +1,9 @@\n 1\n-2\n+X\n 3\n 4\n 5\n 6\n 7\n-8\n+Y\n 9\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := unifiedDiff("f", []byte(tt.old), []byte(tt.updated)); err != nil || got != tt.want {
				t.Errorf("--- want\n%s--- got (%v)\n%s", tt.want, err, got)
			}
		})
	}
}

// CLI.md §2.5: an interrupt stops fmt before it writes: exit 130, the files as they were.
func TestFmtInterrupted(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"project.canon": "project a {\n  canon: \"0.1\"\n}\n", "a/a.canon": "package a\nlet  x:Int=1\n"}
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errs bytes.Buffer
	code := Main(ctx, []string{cmdFmt}, Env{Stdout: &out, Stderr: &errs, Dir: dir})
	got, err := os.ReadFile(filepath.Join(dir, "a", "a.canon"))
	if code != exitInterrupted || err != nil || string(got) != files["a/a.canon"] {
		t.Errorf("exit %d, file %q (%v)", code, got, err)
	}
}
