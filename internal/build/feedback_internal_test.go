package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// DECISIONS 341, IMPLEMENTATION-PLAN §7.6: a memoized run finds a load of an output as a cold run does.
func TestFeedbackUnderMemo(t *testing.T) {
	src := "/// A.\npackage a\n\n/// Note.\n@text(\"note.txt\")\nexport fn note() -> String { return \"n\\n\" }\n\n" +
		"/// Fed.\nlet fed: String = load.text(\"gen/note.txt\")\n\nemit text { out: \"gen\" }\n"
	fsys := roFS{
		"p/project.canon":  srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"p/a/a.canon":      srcFile(src),
		"p/a/gen/note.txt": srcFile("n\n"),
	}
	z := &analyzer{fs: newEditFS(fsys), dir: "/p", cache: NewCache()}
	for range []int{0, 1} {
		warm, cold := z.pair(t)
		for _, a := range []*Analysis{warm, cold} {
			if n := countCode(a, diag.E8026.Def().Code); n != 1 {
				t.Fatalf("refusals %d, findings %v", n, a.Result().List)
			}
		}
		same(t, "feedback", warm, cold)
	}
}
