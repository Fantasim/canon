package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E30, API.md E24: `"name": ""` decodes, and the edit refuses it as a *ValueError.
func TestRenameNameEmptyJSON(t *testing.T) {
	var e canon.Edit
	if err := json.Unmarshal([]byte(`{"base":"","ops":[{"op":"renameName","path":"a:limit","name":""}]}`), &e); err != nil {
		t.Fatalf("API.md E24: %v", err)
	}
	p, _ := openRename(t, nil)
	var ve *canon.ValueError
	if _, err := p.Edit(context.Background(), e); !errors.As(err, &ve) || ve.Expected != "a name" {
		t.Errorf("API.md E30: %v", err)
	}
}

// API.md E35: a named argument that named nothing names the parameter a rename gives its name:
// a capture, its Detail the argument, whether the call ends the file or a let follows it.
func TestRenameNameNamedArgCapture(t *testing.T) {
	src := wireHead + "/// F.\nfn f(x: Int) -> Int {\n  return x\n}\n\n/// V.\nlet v: Int = f(y: 2)\n"
	for _, text := range []string{src, src + "\n/// W.\nlet w: Int = v\n"} {
		opts := project(map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": text})
		p, err := canon.Open("/law", opts)
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		_, err = renameOnce(p, "a:f.x", "y")
		var pe *canon.PathError
		if !errors.Is(err, canon.ErrNameClash) || !errors.As(err, &pe) || !strings.HasPrefix(pe.Detail, "a/a.canon:10:16 would name a:y") {
			t.Errorf("API.md E35: %v", err)
		}
	}
}

// API.md E37, API.md M5: a parameter renamed to the name of another local of its function is
// undone by position, the position of its declaration in the file as written, whose line broke.
func TestRenameNamePositionUndo(t *testing.T) {
	long := strings.Repeat("q", 88)
	src := wireHead + "/// H.\nfn h(x: Int) -> Int {\n  let r = x\n  if r > 1 {\n    let " + long + " = 3\n    return " + long + "\n  }\n  return r\n}\n"
	after, undo := roundTripUndo(t, map[string]string{"a/a.canon": src}, "a:h.x", long)
	if !strings.Contains(after, "fn h(\n  "+long+": Int,\n) -> Int {") || undo != "a/a.canon:6:3" {
		t.Errorf("API.md E37: undo %q of\n%s", undo, after)
	}
}

// API.md E32: a broken view and a broken translation entry of the target's package refuse the
// rename, reason broken, each listed as `<file>:<line>`.
func TestRenameNameBrokenViewAndTranslation(t *testing.T) {
	cases := []struct{ file, add, want string }{
		{"features/renames/renames.view.canon", "\nview Rung {\n  title \"{nothing}\"\n}\n", "features/renames/renames.view.canon:13"},
		{"features/renames/renames.fr.canon", "Mission.title \"{nothing}\"\n", "features/renames/renames.fr.canon:9"},
	}
	for _, c := range cases {
		dir := copyRenames(t)
		writeFile(t, dir, c.file, tree(t, dir)[c.file]+c.add)
		p := openDisk(t, dir, canon.Options{})
		_, err := renameOnce(p, "features.renames:Scratch", "Notes")
		var ne *canon.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != canon.ReasonBroken || ne.Detail != c.want {
			t.Errorf("API.md E32, %s: %v", c.file, err)
		}
	}
}

// API.md E36: a rename writes no output: after a build, the emitted files and canon.lock are
// byte for byte as the build left them.
func TestRenameNameWritesNoOutput(t *testing.T) {
	dir := copyRenames(t)
	p := openDisk(t, dir, canon.Options{})
	ctx := context.Background()
	if _, err := p.Build(ctx, canon.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	built := tree(t, dir)
	if _, ok := built["features/renames/out/missions.json"]; !ok {
		t.Fatalf("the build wrote no output: %v", changedFiles(nil, built))
	}
	if _, err := renameOnce(p, "features.renames:Mission.name", "title"); err != nil {
		t.Fatal(err)
	}
	for _, f := range changedFiles(built, tree(t, dir)) {
		if strings.Contains(f, "/out/") || strings.HasSuffix(f, "canon.lock") {
			t.Errorf("API.md E36: the rename wrote %s", f)
		}
	}
}

// API.md E35: a translation key naming the renamed field after is the capture Detail names.
// API.md E32, DECISIONS 277: a `@files` variable naming nothing breaks its let; the rename refuses.
func TestRenameNameCaptureInQuietPlaces(t *testing.T) {
	src := func(tpl string) string {
		return wireHead + "/// Kind.\nenum Kind { x, y }\n\n/// Item.\nrecord Item {\n  /// K.\n  kind: Kind\n}\n\n/// The items.\n" +
			"@files(\"" + tpl + "\")\nlet items: table Item = {\n  a { kind: x }\n}\n"
	}
	const declared, broken = ", declared at a/a.canon:10:3", "a/a.canon:14"
	cases := []struct {
		name   string
		files  map[string]string
		reason canon.Reason
		want   string
	}{
		{"template", map[string]string{"a/a.canon": src("items/{g}/{id}.canon")}, canon.ReasonBroken, broken},
		{"twice", map[string]string{"a/a.canon": src("items/{g}/{g}/{id}.canon")}, canon.ReasonBroken, broken},
		{"adjacent", map[string]string{"a/a.canon": src("items/{g}{g}/{id}.canon")}, canon.ReasonBroken, broken},
		{"beside the renamed", map[string]string{"a/a.canon": src("items/{g}/{kind}/{id}.canon")}, canon.ReasonBroken, broken},
		{"translation", map[string]string{"a/a.canon": src("items/{kind}/{id}.canon"), "a/a.fr.canon": "package a\ntranslation fr\n\nItem.g \"Genre\"\n"}, "", "a/a.fr.canon:4:6 would name a:g" + declared},
	}
	for _, c := range cases {
		c.files["project.canon"] = "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"
		p, err := canon.Open("/law", project(c.files))
		if err != nil {
			t.Fatal(err)
		}
		_, err = renameOnce(p, "a:Item.kind", "g")
		if got := refusal(err); got != string(c.reason)+" "+c.want {
			t.Errorf("API.md E32, E35, %s: %v, want %s %q", c.name, err, c.reason, c.want)
		}
		_ = p.Close()
	}
}

// refusal is a rename's refusal as `<reason> <detail>`: a NotEditableError's reason, none for a
// name clash; "" for any other outcome.
func refusal(err error) string {
	var ne *canon.NotEditableError
	if errors.As(err, &ne) {
		return string(ne.Reason) + " " + ne.Detail
	}
	var pe *canon.PathError
	if errors.Is(err, canon.ErrNameClash) && errors.As(err, &pe) {
		return " " + pe.Detail
	}
	return ""
}
