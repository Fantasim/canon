package canon_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// valueLaw is a project with every kind of value and origin Value reads (API.md §5.2, §6).
var valueLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": `/// A.
package a

/// A color.
enum Color { red, green }

/// A shape.
variant Shape {
  /// A square.
  square {
    /// Its side.
    side: Int
  }
  /// A circle.
  circle
}

/// A level.
record Level {
  /// Its code.
  code: Int
  /// Its name.
  name: String = "none"
}

/// A server.
record Server {
  /// Its port.
  port: Int = 8765
  /// Its host.
  host: String = "localhost"
}

/// Settings.
record Config {
  /// The server.
  server: Server = {}
  /// A rate.
  rate: Float = 0.5
  /// A delay.
  delay: Duration = 2s
  /// A flag.
  on: Bool = true
  /// An optional note.
  note: String?
}

/// Twice n.
fn twice(n: Int) -> Int {
  return n * 2
}

/// The settings.
let config: Config = {}

/// Levels by id.
let levels: [Level] keyed by code = [{ code: 3, name: "three" }, { code: 7 }]

/// Plain numbers.
let nums: [Int] = [10, 20]

/// A map with keys that are not words.
let names: {String: Int} = { "two words": 2, "one": 1 }

/// A map keyed by color.
let byColor: {Color: Int} = { green: 5 }

/// A table.
let items: table Level = {
  /// The sword.
  sword { code: 1 }
}

/// A shape.
let shape: Shape = square { side: 4 }

/// A computed value.
let doubled: Int = twice(21)

/// A copy with a spread.
let copy: Server = { ...config.server, host: "h" }

/// Read from JSON.
let loaded: Server = load("server.json")

/// A local value, only reachable qualified.
local let hidden: Int = 1

/// A limit, also in b.
let limit: Int = 1

/// Past the end of its list.
let bad: Int = [1, 2][5]

/// Secrets.
record Gen {
  /// A key.
  key: input String? from env "A_KEY"
}

/// The secrets.
let gen: Gen = {}

/// Seven, n calls down.
fn down(n: Int) -> Int {
  return if n == 0 { 7 * 1 } else { down(n - 1) }
}

/// Computed at the bottom of 20 calls.
let deep: Int = down(19)

/// A range.
let span: Range = 0..3

/// Pairs, which have no wire form.
local let ps = [5, 6].enumerate()
`,
	"a/server.json": `{"port": 80}` + "\n",
	"a/dev.layer.canon": `package a
layer dev

amend config {
  server.port: 9000
}
`,
	"b/b.canon": `/// B.
package b

/// A limit, also in a.
let limit: Int = 2

/// Only in b.
let only: Int = 3
`,
}

func openValueLaw(t *testing.T, layers ...string) *canon.Project {
	t.Helper()
	opts := project(valueLaw)
	opts.Layers = layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func mustValue(t *testing.T, p *canon.Project, path string) *canon.Value {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}

// API.md P6, P7, P7a, P8, P9, P10: roots are found as P6 and P7 say, and every path comes back canonical,
// package-qualified.
func TestValueCanonicalPaths(t *testing.T) {
	p := openValueLaw(t)
	for _, c := range []struct{ in, want, text string }{
		{"only", "b:only", "3"},                                        // P6: public, other package; P10
		{"a:hidden", "a:hidden", "1"},                                  // P7: local, qualified
		{"levels[7].name", "a:levels[7].name", "none"},                 // P1, P8: a keyed list by key
		{"levels[#0]", "a:levels[3]", `Level{code: 3, name: "three"}`}, // P4, P8: no [#n] in canonical form
		{"nums[1]", "a:nums[1]", "20"},                                 // P8: a plain list by index
		{"items[sword].code", "a:items.sword.code", "1"},               // P8: a table entry as .key
		{"names[#0]", `a:names["two words"]`, "2"},                     // P9: a key that is not a word
		{`names["one"]`, "a:names[one]", "1"},                          // P2, P9: a word key as a word
		{"byColor[#0]", "a:byColor[green]", "5"},                       // P9: an enum key by member name
		{"a:Color.green", "a:Color.green", "green"},                    // P7a
		{"shape.kind", "a:shape.kind", "square"},                       // P3
		{"config.server.port", "a:config.server.port", "8765"},         // W3 path through defaults
		{"a:config.note", "a:config.note", "none"},                     // an absent optional is none
	} {
		v := mustValue(t, p, c.in)
		if v.Path != c.want || v.Text != c.text || v.String() != c.text {
			t.Errorf("%s: path %q text %q, want %q %q", c.in, v.Path, v.Text, c.want, c.text)
		}
	}
}

// API.md R5, R6, P5, P6, X1, §15: each refusal is a *PathError wrapping its sentinel.
func TestValueErrors(t *testing.T) {
	p := openValueLaw(t)
	for _, c := range []struct {
		path, text string
		want       error
	}{
		{"config..x", "config..x: invalid path: expected a name at byte 7", canon.ErrBadPath},
		{"nums.x", "nums.x: invalid path: segment .x", canon.ErrBadPath},       // P5
		{"nums[5]", "nums[5]: no value at path: segment [5]", canon.ErrNoPath}, // P4
		{"config.nope", "config.nope: no value at path: segment .nope", canon.ErrNoPath},
		{"hidden", "hidden: no value at path: root hidden", canon.ErrNoPath}, // P6: local is not public
		{"nowhere", "nowhere: no value at path: root nowhere", canon.ErrNoPath},
		{"limit", "limit: ambiguous path: a:limit, b:limit", canon.ErrAmbiguousPath},
		{"bad", "bad: value could not be computed: root bad", canon.ErrNoValue},
		{"gen.key", "gen.key: input field has no value at build time: A_KEY", canon.ErrInputField},
		{"config.note.x", "config.note.x: no value at path: segment .x", canon.ErrNoPath},
	} {
		_, err := p.Value(context.Background(), c.path)
		var perr *canon.PathError
		if !errors.Is(err, c.want) || !errors.As(err, &perr) || perr.Op != -1 || err.Error() != c.text {
			t.Errorf("%s: %v, want %v (%q)", c.path, err, c.want, c.text)
			continue
		}
		switch {
		case c.want == canon.ErrAmbiguousPath && !slices.Equal(perr.Candidates, []string{"a:limit", "b:limit"}):
			t.Errorf("%s: candidates %q", c.path, perr.Candidates)
		case c.want == canon.ErrNoValue && (len(perr.Findings) != 1 || perr.Findings[0].Severity != canon.SeverityError):
			t.Errorf("%s: findings %v", c.path, perr.Findings)
		}
	}
}

// API.md §5.2, EVALUATION.md §13, §9.3: origin kinds, Via, Stack, Pointer and Replaced.
func TestValueOrigins(t *testing.T) {
	p := openValueLaw(t, "dev")
	port := mustValue(t, p, "config.server.port").Origin
	if port.Kind != canon.OriginLayer || port.Layer != "dev" || port.File != "a/dev.layer.canon" || port.Line != 5 {
		t.Errorf("port: %+v", port)
	}
	if port.Text != "9000" {
		t.Errorf("port text %q", port.Text)
	}
	if r := port.Replaced; r == nil || r.Kind != canon.OriginDefault || r.Line != 29 || r.Text != "8765" || r.Via == nil || r.Replaced != nil {
		t.Errorf("port replaced: %+v", r)
	}
	// CLI.md §3.7: the default produced the server before the layer amended its port.
	if server := mustValue(t, p, "config.server").Origin; server.Kind != canon.OriginDefault ||
		server.Text != `Server{port: 8765, host: "localhost"}` {
		t.Errorf("config.server: %s %q", server.Kind, server.Text)
	}
	if cfg := mustValue(t, p, "config").Origin; !strings.Contains(cfg.Text, "port: 8765") {
		t.Errorf("config: %q", cfg.Text)
	}
	host := mustValue(t, p, "config.server.host").Origin
	if host.Kind != canon.OriginDefault || host.Replaced != nil || host.Via == nil || host.Via.Kind != canon.OriginDefault {
		t.Errorf("host: %+v", host)
	}
	doubled := mustValue(t, p, "doubled").Origin
	if doubled.Kind != canon.OriginComputed || len(doubled.Stack) != 1 || doubled.Stack[0].Fn != "twice" {
		t.Errorf("doubled: %+v", doubled)
	}
	port80 := mustValue(t, p, "loaded.port").Origin
	if port80.Kind != canon.OriginJSON || port80.Pointer != "/port" || port80.File != "a/server.json" {
		t.Errorf("loaded.port: %+v", port80)
	}
	copied := mustValue(t, p, "copy.port").Origin
	if copied.Kind != canon.OriginSpread || copied.Via == nil || copied.Via.Kind != canon.OriginLayer || copied.Via.Text != "9000" {
		t.Errorf("copy.port: %+v", copied)
	}
	if lit := mustValue(t, p, "nums[0]").Origin; lit.Kind != canon.OriginLiteral || lit.Line != 60 || lit.Col != 20 {
		t.Errorf("nums[0]: %+v", lit)
	}
	if deep := mustValue(t, p, "deep").Origin; len(deep.Stack) != 16 || deep.MoreFrames != 4 || deep.Text != "7" {
		t.Errorf("deep: %d frames, %d more, text %q", len(deep.Stack), deep.MoreFrames, deep.Text)
	}
	if member := mustValue(t, p, "a:Color.red").Origin; member.Kind != "" || member.Replaced != nil {
		t.Errorf("Color.red: %+v", member)
	}
}

// API.md §5.2, §7: Editable is edit's answer: the file a Set writes, or why none.
func TestValueEditable(t *testing.T) {
	p := openValueLaw(t, "dev")
	for _, c := range []struct {
		path string
		want canon.Editability
	}{
		{"nums[0]", canon.Editability{Mode: canon.EditCanon, File: "a/a.canon"}},
		{"loaded.port", canon.Editability{Mode: canon.EditJSON, File: "a/server.json"}},
		{"doubled", canon.Editability{Mode: canon.EditNone, Reason: canon.ReasonComputed}},
		{"config.server.port", canon.Editability{Mode: canon.EditNone, Reason: canon.ReasonLayered, Layer: "dev"}},
		{"items.sword.code", canon.Editability{Mode: canon.EditCanon, File: "a/a.canon"}},
		{"shape.kind", canon.Editability{Mode: canon.EditNone, Reason: canon.ReasonPseudo}},
	} {
		if got := mustValue(t, p, c.path).Editable; got != c.want {
			t.Errorf("%s: %+v, want %+v", c.path, got, c.want)
		}
	}
}

// API.md §5.2: the kind of each value and its typed accessors, which never re-evaluate.
func TestValueAccessors(t *testing.T) {
	p := openValueLaw(t)
	kinds := map[string]canon.ValueKind{
		"config": canon.KindRecord, "config.on": canon.KindBool, "nums[0]": canon.KindInt,
		"config.rate": canon.KindFloat, "config.server.host": canon.KindString,
		"config.delay": canon.KindDuration, "a:Color.red": canon.KindEnum, "shape": canon.KindVariant,
		"nums": canon.KindList, "levels": canon.KindKeyedList, "items": canon.KindTable,
		"names": canon.KindMap, "config.note": canon.KindNone,
	}
	//canon:unordered each path is checked on its own
	for path, want := range kinds {
		if got := mustValue(t, p, path).Kind; got != want {
			t.Errorf("%s: kind %s, want %s", path, got, want)
		}
	}
	on, okB := mustValue(t, p, "config.on").Bool()
	n, okI := mustValue(t, p, "nums[1]").Int()
	f, okF := mustValue(t, p, "config.rate").Float()
	s, okS := mustValue(t, p, "config.server.host").Str()
	d, okD := mustValue(t, p, "config.delay").Dur()
	enum, member, okM := mustValue(t, p, "a:Color.green").Member()
	cs, okC := mustValue(t, p, "shape").Case()
	key, okK := mustValue(t, p, "levels[7]").Key()
	if !on || n != 20 || f != 0.5 || s != "localhost" || d != 2*time.Second || enum != "a.Color" ||
		member != "green" || cs != "square" || key != "7" || !(okB && okI && okF && okS && okD && okM && okC && okK) {
		t.Errorf("accessors: %v %d %v %q %v %q %q %q %q", on, n, f, s, d, enum, member, cs, key)
	}
	if _, ok := mustValue(t, p, "nums[1]").Str(); ok {
		t.Error("Str of an Int")
	}
	if v := mustValue(t, p, "config.note"); !v.IsNone() || v.Len() != 0 || v.Children() != nil {
		t.Errorf("note: %v %d", v.IsNone(), v.Len())
	}
}

// API.md §5.2, R4: Children and Child read the value's own snapshot; JSON is the wire form.
func TestValueChildren(t *testing.T) {
	p := openValueLaw(t)
	cfg := mustValue(t, p, "config")
	var paths []string
	for _, c := range cfg.Children() {
		paths = append(paths, c.Path)
	}
	want := []string{"a:config.server", "a:config.rate", "a:config.delay", "a:config.on", "a:config.note"}
	if cfg.Len() != len(want) || !slices.Equal(paths, want) {
		t.Errorf("children %q, len %d", paths, cfg.Len())
	}
	names := mustValue(t, p, "names")
	if c := names.Children(); len(c) != 2 || c[0].Path != `a:names["two words"]` || c[1].Path != "a:names[one]" {
		t.Errorf("map children: %v", c)
	}
	port, err := cfg.Child(".server")
	if err == nil {
		port, err = port.Child(".port")
	}
	if err != nil || port.Path != "a:config.server.port" || port.Text != "8765" {
		t.Fatalf("Child: %v %v", port, err)
	}
	for _, c := range []struct {
		seg  string
		want error
	}{{".nope", canon.ErrNoPath}, {"..", canon.ErrBadPath}, {".a.b", canon.ErrBadPath}, {"", canon.ErrBadPath}} {
		if _, err := cfg.Child(c.seg); !errors.Is(err, c.want) {
			t.Errorf("Child(%q): %v, want %v", c.seg, err, c.want)
		}
	}
	if got := string(mustValue(t, p, "levels").JSON()); got != `[{"code":3,"name":"three"},{"code":7,"name":"none"}]` {
		t.Errorf("JSON: %s", got)
	}
	if got := string(mustValue(t, p, "items").JSON()); got != `{"sword":{"code":1,"name":"none"}}` {
		t.Errorf("table JSON: %s", got)
	}
	if r := mustValue(t, p, "span"); r.Kind != canon.KindRange || r.JSON() != nil { // WIRE.md §5.9
		t.Errorf("range: %s %s", r.Kind, r.JSON())
	}
	if ps := mustValue(t, p, "a:ps"); ps.JSON() != nil || ps.Children()[0].JSON() != nil {
		t.Errorf("pairs: %s", ps.JSON())
	}
}

// API.md R4: a Value holds its snapshot: a later change to the files leaves it and its children as
// they were, while a new read sees the change.
func TestValueHoldsSnapshot(t *testing.T) {
	opts := project(valueLaw)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	before := mustValue(t, p, "nums")
	if err := opts.FS.WriteFile("/law/b/b.canon", []byte("/// B.\npackage b\n\n/// Nums.\nlet nums: [Int] = [1]\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Value(context.Background(), "nums"); !errors.Is(err, canon.ErrAmbiguousPath) {
		t.Errorf("after the change: %v", err)
	}
	c := before.Children()
	if before.Text != "[10, 20]" || len(c) != 2 || c[1].Text != "20" {
		t.Errorf("held value changed: %s %v", before.Text, c)
	}
}

// API.md §2.1, §7.5: Options.EditLayer is the layer Editable judges with (W11).
func TestValueEditLayer(t *testing.T) {
	opts := project(valueLaw)
	opts.Layers, opts.EditLayer = []string{"dev"}, "dev"
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if e := mustValue(t, p, "config.server.port").Editable; e.Mode != canon.EditCanon || e.File != "a/dev.layer.canon" {
		t.Errorf("edit layer: %+v", e)
	}
}

// API.md R6, EVALUATION.md §7.2: ErrNoValue wraps the recorded cause, the root's own or upstream.
func TestValueNoValueCauses(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     "/// A.\npackage a\n\nimport b\n\n/// Reads b.\nlet total: Int = b.bad + 1\n\n/// Own.\nlet own: Int = [1][2]\n",
		"b/b.canon":     "/// B.\npackage b\n\n/// Bad.\nlet bad: Int = [1, 2][5]\n",
		// Only stage B's where predicate, of a dependent type, fails (EVALUATION.md §5, §7.1).
		"c/c.canon": "/// C.\npackage c\n\n/// K.\nenum K { num, many }\n\n/// P.\ntype P(k: K) = match k {\n" +
			"  num => Int where [1][it] == 1\n  many => [Int]\n}\n\n/// Thing.\nrecord Thing {\n  /// K.\n  k: K\n" +
			"  /// Payload.\n  p: P(k)\n}\n\n/// Things.\nlet things: [Thing] = [{ k: num, p: 3 }]\n",
		// The checker breaks x; y names it (TYPES.md §1): neither is ever evaluated.
		"d/d.canon": "/// D.\npackage d\n\n/// A type error.\nlet x: Int = \"s\"\n",
		"e/e.canon": "/// E.\npackage e\n\nimport d\n\n/// Names d.x.\nlet y: Int = d.x + 1\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, c := range []struct{ path, file string }{{"total", "b/b.canon"}, {"own", "a/a.canon"}, {"things", "c/c.canon"}, {"d:x", "d/d.canon"}, {"e:y", "d/d.canon"}} {
		_, err := p.Value(context.Background(), c.path)
		var perr *canon.PathError
		if !errors.As(err, &perr) || !errors.Is(err, canon.ErrNoValue) || len(perr.Findings) != 1 || perr.Findings[0].File != c.file {
			t.Errorf("%s: %v, findings %+v, want one in %s", c.path, err, perr, c.file)
		}
	}
}
