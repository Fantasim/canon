package lock_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	taxonomyPath = "../../examples/teamboard/taxonomy.canon"
	goldenPath   = "../../examples/teamboard/expected/canon.lock"
	vocabPath    = "../../examples/resource/vocab/vocab.canon"
	vocabPkg     = "resource.vocab"
	vocabSize    = 3995
	vocabSum     = "f71a7d091f356e308652875b83be336cc95ca8cd247cff949c7a6a30bfaea98a"
)

// teamboard reads examples/teamboard/taxonomy.canon, edited by the replacements (old, new…),
// and its current facts.
func teamboard(t *testing.T, edits ...string) (*source.FileSet, *lock.Sources) {
	t.Helper()
	data, err := os.ReadFile(taxonomyPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.NewReplacer(edits...).Replace(string(data))
	fs := &source.FileSet{}
	src, err := fs.Add("teamboard/taxonomy.canon", "/teamboard/taxonomy.canon", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	file := syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "teamboard"))
	return fs, sourcesOf(t, "teamboard", []*syntax.File{file})
}

func goldenLock(t *testing.T) []byte {
	t.Helper()
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	return want
}

// M1 acceptance 5, LOCK.md §9.1: the first build writes the golden lock; a second changes nothing.
func TestTeamboardFirstBuild(t *testing.T) {
	_, s := teamboard(t)
	f := lock.New("teamboard")
	changed, err := f.Update(s)
	if err != nil || !changed || !bytes.Equal(f.Format(), goldenLock(t)) {
		t.Fatalf("Update = %v, %v; lock:\n%s", changed, err, f.Format())
	}
	if again, err := f.Update(s); again || err != nil {
		t.Errorf("a second build changed the lock: %v %v", again, err)
	}
}

// verifyTeamboard compares the golden lock with the edited taxonomy and returns the findings.
func verifyTeamboard(t *testing.T, edits ...string) ([]diag.Finding, *lock.File, *lock.Sources) {
	t.Helper()
	fs, s := teamboard(t, edits...)
	want := goldenLock(t)
	src, err := fs.Add("teamboard/canon.lock", "/teamboard/canon.lock", want)
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fs, "teamboard")
	l, ok := lock.Parse(src.ID, src.Content, "teamboard", bag)
	if !ok {
		t.Fatal(bag.Findings())
	}
	l.Verify(s, bag)
	return bag.Findings(), l, s
}

// M1 acceptance 5, LOCK.md §9.3, §9.5: deleting a status or renaming one is E6001.
func TestTeamboardDeleteAndRename(t *testing.T) {
	const wontDo = "teamboard.statuses.wont_do"
	for _, c := range []struct {
		name  string
		edits []string
		want  *diag.Builder
	}{
		{"delete", []string{"  wont_do { tone: neutral, label: \"Won't do\", terminal: true, next: [open], requires: [reason] }\n", ""},
			diag.E6001.AtRemoved(source.Span{}, wontDo)},
		{"rename", []string{"wont_do", "rejected"},
			diag.E6001.AtRenamed(source.Span{}, wontDo, "rejected", "wont_do")},
	} {
		findings, _, _ := verifyTeamboard(t, c.edits...)
		want := diag.NewBag(&source.FileSet{}, "teamboard")
		c.want.Report(want)
		if len(findings) != 1 || findings[0].Code != want.Findings()[0].Code || findings[0].Message != want.Findings()[0].Message {
			t.Errorf("%s: %v", c.name, findings)
		}
	}
}

// M1 acceptance 5, LOCK.md §9.4: retiring a status passes, and the build records it on its line.
func TestTeamboardRetire(t *testing.T) {
	findings, l, s := verifyTeamboard(t, "  duplicate {", "  retired duplicate {")
	if len(findings) != 0 {
		t.Fatalf("retiring reported %v", findings)
	}
	if n := l.Pending(s, source.Span{}, diag.NewBag(&source.FileSet{}, "teamboard")); n != 0 {
		t.Errorf("pending %d: a retirement is not a new value", n)
	}
	changed, err := l.Update(s)
	want := strings.Replace(string(goldenLock(t)), "statuses  duplicate\n", "statuses  duplicate  retired\n", 1)
	if err != nil || !changed || string(l.Format()) != want {
		t.Errorf("Update = %v, %v; lock:\n%s", changed, err, l.Format())
	}
}

// LOCK.md §9.2: an added status is one new line in canonical order.
func TestTeamboardAdd(t *testing.T) {
	findings, l, s := verifyTeamboard(t, "  open {", "  blocked { tone: warning, label: \"Blocked\", terminal: false, next: [open] }\n  open {")
	if len(findings) != 0 {
		t.Fatalf("adding reported %v", findings)
	}
	if _, err := l.Update(s); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(goldenLock(t)), "table  teamboard.statuses  duplicate\n", "table  teamboard.statuses  blocked\ntable  teamboard.statuses  duplicate\n", 1)
	if string(l.Format()) != want {
		t.Errorf("lock:\n%s", l.Format())
	}
}

// LOCK.md §1: AddTable takes only stable tables, and a key the lock cannot hold is refused.
func TestSourcesRefuse(t *testing.T) {
	row := &types.RecordType{Pkg: "p", Name: "Row"}
	plain := &value.Table{T: &types.TableType{Elem: row}}
	if _, err := lock.NewSources("p").AddTable("p.t", plain, nil); !errors.Is(err, lock.ErrNotLocked) {
		t.Errorf("a plain table: %v", err)
	}
	bad := &value.Table{T: &types.TableType{Elem: row, Stable: true}, Entries: []*value.Record{
		{T: row, Ident: &value.Identity{Key: value.Key{I: 3, IsInt: true}}},
	}}
	if _, err := lock.NewSources("p").AddTable("p.t", bad, nil); !errors.Is(err, lock.ErrBadFact) {
		t.Errorf("an integer key: %v", err)
	}
}

// EVALUATION.md §7.2: a poisoned stable table is not compared: its gone keys report nothing.
func TestSkippedCollection(t *testing.T) {
	fs, s := teamboard(t, "  wont_do {", "  rejected {")
	s.Skip(lock.KindTable, "teamboard.statuses")
	src, err := fs.Add("teamboard/canon.lock", "/teamboard/canon.lock", goldenLock(t))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fs, "teamboard")
	l, _ := lock.Parse(src.ID, src.Content, "teamboard", bag)
	l.Verify(s, bag)
	if n := l.Pending(s, source.Span{}, bag); len(bag.Findings()) != 0 || n != 0 {
		t.Errorf("findings %v, pending %d", bag.Findings(), n)
	}
}

// LOCK.md §9.6: the first build of resource.vocab writes its lock, 3 995 bytes, SHA-256 f71a7d09….
func TestVocabFirstBuild(t *testing.T) {
	data, err := os.ReadFile(vocabPath)
	if err != nil {
		t.Fatal(err)
	}
	fs := &source.FileSet{}
	src, err := fs.Add("resource/vocab/vocab.canon", "/resource/vocab/vocab.canon", data)
	if err != nil {
		t.Fatal(err)
	}
	file := syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, vocabPkg))
	f := lock.New(vocabPkg)
	if _, err := f.Update(sourcesOf(t, vocabPkg, []*syntax.File{file})); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(f.Format())
	if got := hex.EncodeToString(sum[:]); len(f.Format()) != vocabSize || got != vocabSum {
		t.Errorf("%d bytes, sha256 %s:\n%s", len(f.Format()), got, f.Format())
	}
}
