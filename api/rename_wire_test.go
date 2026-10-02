package canon_test

import (
	"context"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// wireHead opens every file of the wire cases.
const wireHead = "/// A.\npackage a\n\n"

// wireRecord is a record of one field `count`, loaded or emitted by the text after it.
func wireRecord(field, rest string) string {
	return wireHead + "/// R.\nrecord R {\n  /// C.\n  " + field + "\n}\n\n" + rest
}

// roundTrip renames name to newName in a project of files, returns a/a.canon as written, and
// fails unless the Undo gives every file back byte for byte (API.md E37).
func roundTrip(t *testing.T, files map[string]string, name, newName string) string {
	after, _ := roundTripUndo(t, files, name, newName)
	return after
}

// roundTripUndo is roundTrip, with the name the Undo renamed back.
func roundTripUndo(t *testing.T, files map[string]string, name, newName string) (string, string) {
	t.Helper()
	files["project.canon"] = "project acme {\n  canon: \"0.1\"\n}\n"
	opts := project(files)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	res, err := renameOnce(p, name, newName)
	if err != nil {
		var found []canon.Finding
		if res != nil {
			found = res.Findings
		}
		t.Fatalf("%s -> %s: %v %+v", name, newName, err, found)
	}
	after := read(t, m, "a/a.canon")
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("undo of %s: %v", name, err)
	}
	for f, text := range files { //canon:unordered each file is compared alone
		if got := read(t, m, f); got != text {
			t.Errorf("API.md E37, undo of %s: %s is %q, was %q", name, f, got, text)
		}
	}
	return after, res.Undo[0].Path
}

// API.md E34: the old wire name, merged first into an existing `@json(…)`, when a load (load.dir,
// load.csv) or an emit writing data (json; ts embedded or types, not baked) reaches the record,
// never through a ref, never beside `path:`.
func TestRenameNameWireReach(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"merged", map[string]string{"a/a.canon": wireRecord("count: Duration @json(unit: s)", "/// V.\nlet v: R = load(\"v.json\")\n"), "a/v.json": "{\n  \"count\": 1\n}\n"}, "  total: Duration @json(\"count\", unit: s)\n"},
		{"load.dir", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: table R = load.dir(\"v/*.json\")\n"), "a/v/x.json": "{\n  \"count\": 1\n}\n"}, "  total: Int @json(\"count\")\n"},
		{"load.csv", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: [R] = load.csv(\"v.csv\", header: true)\n"), "a/v.csv": "count\n1\n"}, "  total: Int @json(\"count\")\n"},
		{"ts embedded", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: R = { count: 1 }\n\nemit ts { out: \"out/a.ts\", mode: embedded }\n")}, "  total: Int @json(\"count\")\n"},
		{"ts types", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: R = { count: 1 }\n\nemit ts { out: \"out/a.ts\", mode: types }\n")}, "  total: Int @json(\"count\")\n"},
		{"ts baked", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: R = { count: 1 }\n\nemit ts { out: \"out/a.ts\" }\n")}, "  total: Int\n"},
		{"through a ref", map[string]string{"a/a.canon": wireRecord("count: Int", "/// V.\nlet v: table R = { x { count: 1 } }\n\n/// H.\nrecord H {\n  /// P.\n  p: ref v\n}\n\n/// The h.\nlet h: H = load(\"h.json\")\n"), "a/h.json": "{\n  \"p\": \"x\"\n}\n"}, "  total: Int\n"},
		{"path", map[string]string{"a/a.canon": wireRecord("count: Int @json(path: \"a.b\")", "/// V.\nlet v: R = load(\"v.json\")\n"), "a/v.json": "{\n  \"a\": {\n    \"b\": 1\n  }\n}\n"}, "  total: Int @json(path: \"a.b\")\n"},
	}
	for _, c := range cases {
		if got := roundTrip(t, c.files, "a:R.count", "total"); !strings.Contains(got, c.want) {
			t.Errorf("API.md E34, %s: no %q in\n%s", c.name, c.want, got)
		}
	}
}

// API.md E34: a field `inline` on the wire, and a `pairs:` list, keep their text: no wire name.
func TestRenameNameWireExclusions(t *testing.T) {
	inline := wireHead + "/// K.\nvariant K {\n  /// A.\n  a {\n    /// N.\n    n: Int\n  }\n}\n\n/// R.\nrecord R {\n  /// K.\n  kind: K @json(inline)\n}\n\n/// V.\nlet v: R = load(\"v.json\")\n"
	if got := roundTrip(t, map[string]string{"a/a.canon": inline, "a/v.json": "{\n  \"kind\": \"a\",\n  \"n\": 1\n}\n"}, "a:R.kind", "sort"); !strings.Contains(got, "  sort: K @json(inline)\n") {
		t.Errorf("API.md E34, inline: %s", got)
	}
	pairs := wireHead + "/// P.\nrecord P {\n  /// K.\n  k: Int\n  /// W.\n  w: Int\n}\n\n/// R.\nrecord R {\n  /// Ps.\n  ps: [P](..=2) @json(pairs: [\"k{i}\", \"w{i}\"])\n}\n\n/// V.\nlet v: R = load(\"v.json\")\n"
	if got := roundTrip(t, map[string]string{"a/a.canon": pairs, "a/v.json": "{\n  \"k0\": 1,\n  \"w0\": 2\n}\n"}, "a:R.ps", "items"); !strings.Contains(got, "  items: [P](..=2) @json(pairs: [\"k{i}\", \"w{i}\"])\n") {
		t.Errorf("API.md E34, pairs: %s", got)
	}
}

// API.md E34, API.md E37: under `case: snake` a redundant `@json("max_players")` is kept by the
// rename, then dropped by the reverse rename, its default giving it back: the wire is unchanged.
func TestRenameNameRedundantWireName(t *testing.T) {
	src := wireHead + "/// S.\nrecord S @json(case: snake) {\n  /// Max.\n  maxPlayers: Int @json(\"max_players\")\n}\n\n/// The s.\nlet s: S = load(\"s.json\")\n"
	files := map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": src, "a/s.json": "{\n  \"max_players\": 3\n}\n"}
	opts := project(files)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	m, _ := opts.FS.(*memFS)
	ctx := context.Background()
	res, err := renameOnce(p, "a:S.maxPlayers", "cap")
	if err != nil || !strings.Contains(read(t, m, "a/a.canon"), "  cap: Int @json(\"max_players\")\n") {
		t.Fatalf("API.md E34: %v\n%s", err, read(t, m, "a/a.canon"))
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, m, "a/a.canon"); got != strings.Replace(src, " @json(\"max_players\")", "", 1) {
		t.Errorf("API.md E37: the redundant wire name is not dropped:\n%s", got)
	}
	v, err := p.Value(ctx, "a:s.maxPlayers")
	if err != nil || v.Text != "3" {
		t.Errorf("API.md E34: the wire reads %v, %v", v, err)
	}
}
