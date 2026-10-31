package tsgen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	unselectedBaked = "testdata/ts/ids_unselected.txtar"
	unselectedData  = "testdata/ts/ids_unselected_data.txtar"
	unselectedA     = "features/a/a.ts"
	unselectedFn    = "export function isTop"
)

// generatedOf is the content of one generated file of a txtar case's sources.
func generatedOf(t *testing.T, path, file string) string {
	t.Helper()
	cases, err := golden.Load(path, golden.Expected(generatedTS))
	if err != nil {
		t.Fatal(err)
	}
	w := buildWorld(t, cases[0].Archive.Files)
	for _, o := range w.generate(t) {
		if o.Path == file {
			return string(o.Content)
		}
	}
	t.Fatalf("%s: no %s", path, file)
	return ""
}

// CODEGEN.md §2.1, §5.3, DECISIONS 413: a table `values` leaves out still has its id type, Members and private index, written before the export fn that names them.
func TestUnselectedTableKeepsItsIds(t *testing.T) {
	text := generatedOf(t, unselectedBaked, unselectedA)
	at := strings.Index(text, unselectedFn)
	if at < 0 {
		t.Fatalf("no %s in %s", unselectedFn, unselectedA)
	}
	for _, want := range []string{"export type TierId =", "export const TierIdMembers", "export const TierIdLiveMembers", "const TierIdIndex"} {
		if i := strings.Index(text, want); i < 0 || i > at {
			t.Errorf("%q missing or after isTop", want)
		}
	}
}

// CODEGEN.md §2.1, §5.3, DECISIONS 413: in data mode an unselected table's id is the string, declared before the record whose field names it.
func TestUnselectedTableKeepsItsIdsData(t *testing.T) {
	text := generatedOf(t, unselectedData, unselectedA)
	at := strings.Index(text, "export interface Other")
	i := strings.Index(text, "export type TierId = string;")
	if at < 0 || i < 0 || i > at {
		t.Errorf("TierId missing or after Other in data mode")
	}
	if !strings.Contains(text, "readonly tier: TierId;") {
		t.Errorf("Other.tier does not name TierId")
	}
}
