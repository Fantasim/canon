package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"golang.org/x/tools/txtar"
)

// DECISIONS 195, EVALUATION.md §3.3: implicit frames stop a too deep chain at E4402; one within the limit evaluates.
func TestImplicitFrames(t *testing.T) {
	tests := []struct {
		file string
		root string
		want bool // the value evaluates
	}{
		{"E4402_3.txtar", "config", false},
		{"E4402_4.txtar", "deep", false},
		{"E4402_4.txtar", "shallow", true},
		{"E4402_5.txtar", "r", false},
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.root, func(t *testing.T) {
			a, err := txtar.ParseFile(filepath.Join("testdata", "findings", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			b := runBuild(t, fromArchive(t, a), eval.Options{})
			_, got := b.values[eval.Root{Pkg: "a", Name: tt.root}]
			if got != tt.want {
				t.Errorf("a.%s evaluated: %v, want %v", tt.root, got, tt.want)
			}
		})
	}
}
