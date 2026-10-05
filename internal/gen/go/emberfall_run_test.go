package gogen_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

const (
	emberGoldens = "testdata/emberfall"
	emberWorld   = "game/world/out/go"
)

// emberFiles is game.core and game.world generated, world in mode.
func emberFiles(t *testing.T, mode ir.Mode) map[string][]byte {
	t.Helper()
	c, w := emberfall(mode)
	return generateData(t, c.p, w)
}

// CODEGEN.md §5.14, §5.9, §2.8 (DECISIONS 323): game.core writes its make hooks; game.world builds core's values through them, baked as hook calls, in data mode with its own readers, and holds core's table records in its own row type.
func TestEmberfallGoldens(t *testing.T) {
	for name, mode := range map[string]ir.Mode{"baked": ir.ModeBaked, "data": ir.ModeData} { //canon:unordered each golden alone
		files := emberFiles(t, mode)
		checkGoldens(t, files, sortedPaths(files), filepath.Join(emberGoldens, name))
	}
}

// CODEGEN.md §5.14, §5.9: baked game.world compiles and runs: its values, rows, IDs, Record() and core's resolved refs.
func TestEmberfallBakedRuns(t *testing.T) {
	runData(t, emberFiles(t, ir.ModeBaked), emberWorld, "testdata/smoke/emberfall_baked_test.go", nil)
}

// CODEGEN.md §2.8, §5.9, §5.14: data game.world reads core's classes with its own readers and core's hooks, and refuses a ref core resolves to no entry with `no entry`.
func TestEmberfallDataRuns(t *testing.T) {
	runData(t, emberFiles(t, ir.ModeData), emberWorld, "testdata/smoke/emberfall_data_test.go", readData(t, "testdata/datafiles/emberfall"))
}
