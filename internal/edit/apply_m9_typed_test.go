package edit_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// numNormal is numLoose once normalized, with the float added by the edit of the tests below.
const numNormal = "{\n  \"fn0\": \"a\",\n  \"fx0\": 1.5,\n  \"fn1\": \"b\",\n  \"fx1\": 1,\n  \"fn2\": \"c\",\n  \"fx2\": 3,\n  \"dn0\": \"w\",\n  \"dd0\": 5\n}\n"

// API.md M9, FORMATTER.md 14.1 (log-2026-09-29 M4 B11-r2): a JSON source whose typed numbers
// `canon fmt --json-sources` would write otherwise is not canonical: the edit normalizes it
// first and names it, for the caller to refuse without Normalize.
func TestM9TypedNumbers(t *testing.T) {
	ops := []edit.Operation{{Kind: edit.OpAdd, Path: "t.fs", Value: edit.Obj{"n": edit.Str("c"), "x": edit.Float(3)}}}
	for _, c := range []struct {
		name, json string
		loose      bool
	}{
		{"canonical numbers", numCanon, false},
		{"numbers written otherwise", numLoose, true},
	} {
		s := open(t, numFS(c.json), nil, "", "d")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := len(plan.NotCanonical) == 1 && plan.NotCanonical[0] == "d/t.json"; got != c.loose {
			t.Errorf("%s: NotCanonical %q", c.name, plan.NotCanonical)
		}
		if got := string(written(numFS(c.json), plan)["law/d/t.json"].Data); got != numNormal {
			t.Errorf("%s: t.json is\n%s\nwant\n%s", c.name, got, numNormal)
		}
	}
}

// API.md M9 (log-2026-09-29 M4 B11-r2): through the API, the source with numbers written otherwise
// is refused and left as it is without Normalize, and normalized as part of the edit with it.
func TestM9TypedNumbersRefused(t *testing.T) {
	dir := t.TempDir()
	for name, f := range numFS(numLoose) { //canon:unordered each file is written alone
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, filePerm); err != nil {
			t.Fatal(err)
		}
	}
	p, err := canon.Open(filepath.Join(dir, "law"), canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	e := canon.Edit{Ops: []canon.Op{canon.Add("d:t.fs", canon.Obj{"n": canon.Str("c"), "x": canon.Float(3)})}}
	var nc *canon.NotCanonicalError
	if _, err := p.Edit(context.Background(), e); !errors.As(err, &nc) || !slices.Equal(nc.Files, []string{"d/t.json"}) {
		t.Fatalf("Edit: %v, want d/t.json not canonical", err)
	}
	tjson := filepath.Join(dir, "law", "d", "t.json")
	if got, _ := os.ReadFile(tjson); string(got) != numLoose {
		t.Errorf("a refused edit wrote:\n%s", got)
	}
	e.Normalize = true
	if _, err := p.Edit(context.Background(), e); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got, _ := os.ReadFile(tjson); string(got) != numNormal {
		t.Errorf("Normalize: t.json is\n%s\nwant\n%s", got, numNormal)
	}
}

// API.md M9 (log-2026-09-29 M4 B11-r2): of the JSON sources the examples load, exactly the six
// kept in their real layout (jsonsrc's notNormalized) are not canonical, as before.
func TestM9Examples(t *testing.T) {
	dir, roots := exampleRoots(t)
	p, err := build.Open(project.OS(), dir, build.Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := edit.NewSnapshot(a)
	var loose []string
	for display, raw := range edit.JSONSources(snap) { //canon:unordered sorted below
		out, err := edit.CanonicalJSON(snap, display, raw)
		if err != nil {
			t.Fatalf("%s: %v", display, err)
		}
		if !bytes.Equal(out, raw) {
			loose = append(loose, display)
		}
	}
	slices.Sort(loose)
	want := []string{
		"@resource/Server/Item/propItem.json", "@resource/Server/Npc/etc.json", "@resource/Server/Skill/skills.json",
		"features/codes/data/guild_rights.json", "features/embedded/data/countries.json", "features/legacycpp/data/props.json",
	}
	if !slices.Equal(loose, want) {
		t.Errorf("not canonical: %q, want %q", loose, want)
	}
}

// The modes of the files and directories the API test writes.
const (
	dirPerm  = 0o755
	filePerm = 0o644
)

// twoReadsSrc reads n.json as a map of Float and, when second is given, again as a map of it.
func twoReadsSrc(second string) string {
	src := "/// P.\npackage d\n\n/// As Float.\nlet a: {String: Float} = load(\"n.json\")\n"
	if second != "" {
		src += "\n/// Again.\nlet b: {String: " + second + "} = load(\"n.json\")\n"
	}
	return src
}

// API.md M9 (log-2026-09-29 M4 B11-r3): a number token two loads read as different types is left
// as written, neither normalized nor a layout to refuse; read as one type, it is normalized.
func TestM9TwoReadings(t *testing.T) {
	for _, c := range []struct {
		name, second, json, want string
	}{
		{"one reading", "", "{\n  \"x\": 1.0\n}\n", "{\n  \"x\": 1\n}\n"},
		{"Float and Float32", "Float32", "{\n  \"x\": 1.0\n}\n", "{\n  \"x\": 1.0\n}\n"},
		{"Float and Int", "Int", "{\n  \"x\": -0\n}\n", "{\n  \"x\": -0\n}\n"},
		{"Float alone, -0", "", "{\n  \"x\": -0\n}\n", "{\n  \"x\": 0\n}\n"},
	} {
		fsys := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(twoReadsSrc(c.second)), "law/d/n.json": file(c.json)}
		s := open(t, fsys, nil, "", "d")
		out, err := edit.CanonicalJSON(s.snap, "d/n.json", []byte(c.json))
		if err != nil || string(out) != c.want {
			t.Errorf("%s: %q, %v; want %q", c.name, out, err, c.want)
		}
	}
}
