package integrity

import (
	"strings"
	"testing"
)

const droppedDiff = `diff --git a/internal/db/images.go b/internal/db/images.go
--- a/internal/db/images.go
+++ b/internal/db/images.go
@@ -21,4 +21,1 @@ package db
-// WHO MAY ATTACH: the author or a triager. This is an interpretation,
-// not in DOCTRINE; a GM who can only see a post cannot write to it.
-//
-// It was rejected to let any viewer attach.
+// authorizeImageUpload admits the author or a triager.
@@ -80,3 +77,0 @@ func x() {
-	// Reflowed, a judgement call: the same
-	// words come back below.
-	y := 1
@@ -90,0 +88,1 @@ func z() {
+	// Reflowed, a judgement call: the same words, now one line.
`

func TestParseDiffFindsRemovedCommentRuns(t *testing.T) {
	blocks, added := parseDiff(droppedDiff)
	if len(blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (%+v)", len(blocks), blocks)
	}
	if blocks[0].file != "internal/db/images.go" || blocks[0].line != 21 || len(blocks[0].lines) != 4 {
		t.Errorf("first block = %+v, want images.go:21, 4 lines", blocks[0])
	}
	if !strings.Contains(added["internal/db/images.go"], "judgement call") {
		t.Errorf("added text lost the re-added comment: %q", added)
	}
}

func TestDroppedDecisionIgnoresReaddedWording(t *testing.T) {
	blocks, added := parseDiff(droppedDiff)
	file := added["internal/db/images.go"]
	if got := droppedDecision(blocks[0], file); !strings.Contains(got, "an interpretation") {
		t.Errorf("a deliberate rule cut to nothing reported %q, want its deliberate line", got)
	}
	if got := droppedDecision(blocks[1], file); got != "" {
		t.Errorf("a reflowed decision whose wording came back was reported: %q", got)
	}
}

func TestDroppedDecisionIgnoresPlainExplanations(t *testing.T) {
	b := droppedBlock{lines: []string{"// sweep drops expired windows", "// and caps the map."}}
	if got := droppedDecision(b, ""); got != "" {
		t.Errorf("an explanation with no decision wording was reported: %q", got)
	}
}

func fakeRecordsGit(status, log string) func(...string) (string, error) {
	return func(args ...string) (string, error) {
		switch args[0] {
		case gitStatus:
			return status, nil
		case gitLog:
			return log, nil
		}
		return "", nil
	}
}

func TestRecordsChangedByDecisionsInDiffOrWorktree(t *testing.T) {
	rec := droppedDiff + "+++ b/DECISIONS.md\n"
	if !recordsChanged(fakeRecordsGit("", ""), rec) {
		t.Error("DECISIONS.md edited in the diff did not count")
	}
	if !recordsChanged(fakeRecordsGit(" M DECISIONS.md\n", ""), droppedDiff) {
		t.Error("an uncommitted DECISIONS.md edit did not count")
	}
	if recordsChanged(fakeRecordsGit("", ""), droppedDiff) {
		t.Error("a diff touching only code counted as a record change")
	}
}

func TestDeclaredObsoleteByCommitMessage(t *testing.T) {
	if !declaredObsolete(fakeRecordsGit("", "refactor: drop the old gate\n\nsovaudit:decision-obsolete\n"), "base") {
		t.Error("the commit marker was not honoured")
	}
	if declaredObsolete(fakeRecordsGit("", "refactor: tidy\n"), "base") {
		t.Error("a commit without the marker counted")
	}
}

func TestDroppedDecisionNotHiddenByAnUnrelatedMarker(t *testing.T) {
	b := droppedBlock{lines: []string{"// RequireTOTP is false, Louis's call: GMs are not enrolled."}}
	if got := droppedDecision(b, "// the retry is Louis's call too, for other reasons\n"); got == "" {
		t.Error("an unrelated 'deliberate' elsewhere in the file hid a dropped decision")
	}
}

func TestDroppedDecisionIgnoresIntensifiers(t *testing.T) {
	b := droppedBlock{lines: []string{"// It is deliberately a separate function, on purpose and by design."}}
	if got := droppedDecision(b, ""); got != "" {
		t.Errorf("'deliberately'/'on purpose' alone was reported as a decision: %q", got)
	}
}
