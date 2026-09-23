package lock_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/source"
)

// oneFile is a diag.Files holding one file, id 1.
type oneFile struct {
	path    string
	content []byte
}

func (f oneFile) Path(id source.FileID) string {
	if id != 1 {
		return ""
	}
	return f.path
}

func (f oneFile) Content(source.FileID) []byte { return f.content }

func (f oneFile) Position(_ source.FileID, p source.Pos) (line, col int) {
	before := f.content[:min(int(p), len(f.content))]
	return bytes.Count(before, []byte("\n")) + 1, len(before) - bytes.LastIndexByte(before, '\n')
}

// parse reads data as pkg's lock and returns it with the findings it reported.
func parse(t *testing.T, pkg string, data []byte) (*lock.File, bool, []diag.Finding) {
	t.Helper()
	files := oneFile{path: strings.ReplaceAll(pkg, ".", "/") + "/canon.lock", content: data}
	bag := diag.NewBag(files, pkg)
	f, ok := lock.Parse(1, data, pkg, bag)
	return f, ok, bag.Findings()
}

// sample is a LOCK.md sample: the code block after a heading, and the byte count and
// SHA-256 the paragraph before the block states.
type sample struct {
	pkg, text string
	size      int
	sum       string
}

var reSample = regexp.MustCompile("(?s)exactly these ([0-9 ]+) bytes \\(SHA-256\\s+`([0-9a-f]{64})`\\):\\s*```\n(.*?)```|\\(([0-9 ]+) bytes, SHA-256\\s+`([0-9a-f]{64})`\\).*?```\n(.*?)```")

func lockSamples(t *testing.T) []sample {
	t.Helper()
	doc, err := os.ReadFile("../../spec/LOCK.md")
	if err != nil {
		t.Fatal(err)
	}
	var out []sample
	for _, m := range reSample.FindAllStringSubmatch(string(doc), -1) {
		size, sum, text := m[1]+m[4], m[2]+m[5], m[3]+m[6]
		n, err := strconv.Atoi(strings.ReplaceAll(size, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		pkg := strings.Fields(strings.Split(text, "\n")[1])[1]
		pkg = pkg[:strings.LastIndex(pkg, ".")]
		out = append(out, sample{pkg: pkg, text: text, size: n, sum: sum})
	}
	if len(out) != 2 {
		t.Fatalf("found %d samples in LOCK.md, want §9.1 and §9.6", len(out))
	}
	return out
}

// LOCK.md §9.1, §9.6: each sample prints back byte for byte, with its stated size and hash.
func TestSamplesPrintBackByteExact(t *testing.T) {
	for _, s := range lockSamples(t) {
		f, ok, findings := parse(t, s.pkg, []byte(s.text))
		if !ok || len(findings) != 0 {
			t.Fatalf("%s: %v", s.pkg, findings)
		}
		got := f.Format()
		sum := sha256.Sum256(got)
		if string(got) != s.text || len(got) != s.size || hex.EncodeToString(sum[:]) != s.sum {
			t.Errorf("%s: %d bytes, sha256 %x, want %d and %s:\n%s", s.pkg, len(got), sum, s.size, s.sum, got)
		}
	}
}

// LOCK.md §9.1: the teamboard golden is the canonical form of its own facts.
func TestTeamboardGoldenByteExact(t *testing.T) {
	want, err := os.ReadFile("../../examples/teamboard/expected/canon.lock")
	if err != nil {
		t.Fatal(err)
	}
	f, ok, findings := parse(t, "teamboard", want)
	if !ok || len(findings) != 0 || !bytes.Equal(f.Format(), want) {
		t.Errorf("findings %v; printed:\n%s", findings, f.Format())
	}
	if len(f.Facts()) != 32 {
		t.Errorf("%d facts, want 32", len(f.Facts()))
	}
}

// LOCK.md §2.4: CR LF, blanks, space runs, any order, duplicates and retired copies are read.
func TestReadingIsTolerant(t *testing.T) {
	in := "\r\n  \n# canon.lock v1\r\n" +
		"table    p.t  b  retired\n" +
		"field  p.t.code  \"\\u0061\"  b\n" +
		"enum  p.E  10  TEN\n" +
		"table  p.t  b\n" +
		"\n" +
		"enum  p.E  5  FIVE  retired\r\n" +
		"field  p.t.code  \"a b\"  c\n" +
		"table  p.t  a\n" +
		"table  p.t  a\n" +
		"enum  p.E  -0  ZERO"
	f, ok, findings := parse(t, "p", []byte(in))
	if !ok || len(findings) != 0 {
		t.Fatalf("findings %v", findings)
	}
	want := "# canon.lock v1\n" +
		"enum   p.E  0  ZERO\n" +
		"enum   p.E  5  FIVE  retired\n" +
		"enum   p.E  10  TEN\n" +
		"field  p.t.code  \"a\"  b\n" +
		"field  p.t.code  \"a b\"  c\n" +
		"table  p.t  a\n" +
		"table  p.t  b  retired\n"
	if got := string(f.Format()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// LOCK.md §9.2, §9.4: Add inserts in canonical order, retires in place, ignores a copy.
func TestAddKeepsCanonicalOrder(t *testing.T) {
	f := lock.New("teamboard")
	status := func(key string, retired bool) lock.Fact {
		return lock.Fact{Kind: lock.KindTable, Name: "teamboard.statuses", Holder: key, Retired: retired}
	}
	var changes []bool
	for _, fact := range []lock.Fact{
		status("open", false),
		status("duplicate", false),
		{Kind: lock.KindTable, Name: "teamboard.severities", Holder: "normal"},
		status("blocked", false),
		status("duplicate", true),
		status("duplicate", false),
		{Kind: lock.KindField, Name: "teamboard.statuses", Field: "rank", Value: lock.Value{Int: 2}, Holder: "open"},
		{Kind: lock.KindField, Name: "teamboard.statuses", Field: "code", Value: lock.Value{IsString: true, Str: "x"}, Holder: "open"},
		{Kind: lock.KindEnum, Name: "teamboard.Tone", Value: lock.Value{Int: 9}, Holder: "N"},
	} {
		changed, err := f.Add(fact)
		if err != nil {
			t.Fatalf("Add(%+v): %v", fact, err)
		}
		changes = append(changes, changed)
	}
	if want := []bool{true, true, true, true, true, false, true, true, true}; !slices.Equal(changes, want) {
		t.Errorf("Add reported %v, want %v", changes, want)
	}
	want := "# canon.lock v1\n" +
		"enum   teamboard.Tone  9  N\n" +
		"field  teamboard.statuses.code  \"x\"  open\n" +
		"field  teamboard.statuses.rank  2  open\n" +
		"table  teamboard.severities  normal\n" +
		"table  teamboard.statuses  blocked\n" +
		"table  teamboard.statuses  duplicate  retired\n" +
		"table  teamboard.statuses  open\n"
	if got := string(f.Format()); got != want || f.Package != "teamboard" {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if lock.KindEnum.String() != "enum" || lock.Kind(3).String() != "" {
		t.Error("kind names")
	}
}

// LOCK.md §2.2: Add refuses a fact Parse could not read back, and keeps the set unchanged.
func TestAddRefuses(t *testing.T) {
	table := lock.Fact{Kind: lock.KindTable, Name: "a.b.t", Holder: "k"}
	with := func(edit func(*lock.Fact)) lock.Fact {
		f := table
		edit(&f)
		return f
	}
	refused := []lock.Fact{
		with(func(f *lock.Fact) { f.Kind = 3 }),
		with(func(f *lock.Fact) { f.Name = "a.t" }),
		with(func(f *lock.Fact) { f.Name = "a.b.t.u" }),
		with(func(f *lock.Fact) { f.Name = "x.b.t" }),
		with(func(f *lock.Fact) { f.Name = "a.b.1t" }),
		with(func(f *lock.Fact) { f.Name = "a.b._" }),
		with(func(f *lock.Fact) { f.Field = "f" }),
		with(func(f *lock.Fact) { f.Value.Int = 3 }),
		with(func(f *lock.Fact) { f.Value = lock.Value{IsString: true} }),
		with(func(f *lock.Fact) { f.Holder = "" }),
		with(func(f *lock.Fact) { f.Holder = "a b" }),
		with(func(f *lock.Fact) { f.Holder = `"k"` }),
		{Kind: lock.KindEnum, Name: "a.b.E", Value: lock.Value{IsString: true, Str: "x"}, Holder: "M"},
		{Kind: lock.KindEnum, Name: "a.b.E", Value: lock.Value{Int: 1, Str: "x"}, Holder: "M"},
		{Kind: lock.KindField, Name: "a.b.t", Value: lock.Value{Int: 1}, Holder: "k"},
		{Kind: lock.KindField, Name: "a.b.t", Field: "f.g", Value: lock.Value{Int: 1}, Holder: "k"},
		{Kind: lock.KindField, Name: "a.b.t", Field: "f", Value: lock.Value{Int: 1}, Holder: "k", Retired: true},
		{Kind: lock.KindField, Name: "a.b.t", Field: "f", Value: lock.Value{IsString: true, Str: "\xff"}, Holder: "k"},
		{Kind: lock.KindField, Name: "a.b.t", Field: "f", Value: lock.Value{IsString: true, Int: 2, Str: "x"}, Holder: "k"},
	}
	f := lock.New("a.b")
	for _, fact := range refused {
		if changed, err := f.Add(fact); changed || !errors.Is(err, lock.ErrBadFact) {
			t.Errorf("Add(%+v) = %v, %v; want ErrBadFact", fact, changed, err)
		}
	}
	if n := len(f.Facts()); n != 0 {
		t.Errorf("a refused fact was kept: %d facts", n)
	}
	if _, err := lock.New("").Add(lock.Fact{Kind: lock.KindTable, Name: ".t", Holder: "k"}); !errors.Is(err, lock.ErrBadFact) {
		t.Errorf("a lock of no package accepted %v", err)
	}
}

// .claude/rules/go.md §5: a lock that reads without findings prints a canonical fixed point.
func FuzzParse(f *testing.F) {
	f.Add([]byte("# canon.lock v1\ntable  p.t  a  retired\nenum  p.E  -3  M\nfield  p.t.f  \"x y\"  a\n"))
	f.Add([]byte("\r\n# canon.lock v2\n"))
	f.Add([]byte("<<<<<<< HEAD\n# canon.lock v1\nfield  p.t.f  1  a\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		first, ok, _ := parse(t, "p", data)
		if !ok {
			return
		}
		again, ok, findings := parse(t, "p", first.Format())
		if !ok || !bytes.Equal(again.Format(), first.Format()) {
			t.Fatalf("%q: reread %v %v:\n%s\nthen\n%s", data, ok, findings, first.Format(), again.Format())
		}
	})
}
