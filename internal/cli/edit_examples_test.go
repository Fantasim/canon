package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// editRequests are requests against examples/features/edits, each of one kind of write (API.md §8).
var editRequests = map[string]string{
	"set":        `{"ops":[{"op":"set","path":"features.edits:quests.slay.level","value":5}]}`,
	"reset":      `{"ops":[{"op":"reset","path":"features.edits:quests.slay.repeatable"}]}`,
	"addEntry":   `{"ops":[{"op":"addEntry","path":"features.edits:quests","key":"hunt","source":"{ goal: kill, target: \"bear\", reward: gold { amount: 5 } }"}]}`,
	"rename":     `{"ops":[{"op":"rename","path":"features.edits:quests.slay","key":"hunt"}]}`,
	"setCase":    `{"ops":[{"op":"setCase","path":"features.edits:quests.slay.reward","case":"item","source":"{ id: \"axe\" }"}]}`,
	"json":       `{"ops":[{"op":"set","path":"features.edits:limits.players","value":99}]}`,
	"several":    `{"ops":[{"op":"set","path":"features.edits:server.retries","value":9},{"op":"set","path":"features.edits:server.label","value":"X"}]}`,
	"normalized": `{"normalize":true,"ops":[{"op":"set","path":"features.edits:base.retries","value":6}]}`,
}

// valueRequest is a request whose undo restores values, not text (API.md E22), and the entry whose value is compared.
type valueRequest struct {
	request, entry string
	dropped        bool
}

// valueRequests are the requests of examples/features/edits that change layout or cascade.
var valueRequests = map[string]valueRequest{
	"remove":    {`{"ops":[{"op":"remove","path":"features.edits:quests.gather"}]}`, "quests.gather", false},
	"setAbsent": {`{"ops":[{"op":"set","path":"features.edits:quests.gather.level","value":5}]}`, "quests.gather", false},
	"spread":    {`{"ops":[{"op":"set","path":"features.edits:derived.retries","value":9}]}`, "derived", false},
	"minLevel":  {`{"ops":[{"op":"set","path":"features.edits:quests.slay.minLevel","value":1}]}`, "quests.slay", false},
	"dependent": {`{"ops":[{"op":"set","path":"features.edits:quests.slay.goal","value":"collect"}]}`, "quests.slay", true},
}

// snapshot is every file under dir by slash path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, rerr := os.ReadFile(name)
		rel, _ := filepath.Rel(dir, name)
		out[filepath.ToSlash(rel)] = string(data)
		return rerr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// editRun runs canon edit on the project in dir, the request on stdin.
func editRun(t *testing.T, dir string, roots []string, request string) (int, string, string) {
	t.Helper()
	args := append([]string{"edit", "--project", dir}, roots...)
	var out, errs bytes.Buffer
	code := cli.Main(context.Background(), args, cli.Env{Stdin: strings.NewReader(request), Stdout: &out, Stderr: &errs, Dir: dir})
	return code, out.String(), errs.String()
}

// exampleRootsIn redirects the roots of a copy of examples/project.canon, the written ones to one new directory.
func exampleRootsIn(t *testing.T) []string {
	t.Helper()
	out := t.TempDir()
	args := []string{"--root", "resource=_fixtures/resource", "--root", "client=_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		args = append(args, "--root", name+"="+filepath.ToSlash(filepath.Join(out, name)))
	}
	return args
}

// CLI.md §3.15, API.md §8.8: the printed undo applies and every file is byte-identical again.
func TestEditUndoRoundTrip(t *testing.T) {
	for name, request := range editRequests {
		t.Run(name, func(t *testing.T) {
			dir := copyExamples(t)
			roots := exampleRootsIn(t)
			before := snapshot(t, dir)
			code, out, errs := editRun(t, dir, roots, request)
			if code != 0 || errs != "" {
				t.Fatalf("edit: exit %d, %s%s", code, out, errs)
			}
			first, _, _ := strings.Cut(out, "\n")
			o := parseEdit(t, out)
			if !o.Edit.Applied {
				t.Fatalf("not applied: %s", first)
			}
			if equalFiles(before, snapshot(t, dir)) {
				t.Fatal("the edit wrote nothing")
			}
			code, out, errs = editRun(t, dir, roots, string(o.Edit.Undo))
			if code != 0 || errs != "" {
				t.Fatalf("undo: exit %d, %s%s", code, out, errs)
			}
			if after := snapshot(t, dir); !equalFiles(before, after) {
				t.Errorf("files differ after undo of %s", request)
			}
		})
	}
}

func equalFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}

// explained is the value `canon explain` prints for entry in the project in dir.
func explained(t *testing.T, dir string, roots []string, entry string) string {
	t.Helper()
	args := append([]string{"explain", "--project", dir, "--format", "json"}, roots...)
	var out, errs bytes.Buffer
	code := cli.Main(context.Background(), append(args, "features.edits:"+entry), cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	if code != 0 {
		t.Fatalf("explain: exit %d, %s", code, errs.String())
	}
	var line struct {
		Explain struct {
			Value json.RawMessage `json:"value"`
		} `json:"explain"`
	}
	if err := json.Unmarshal(out.Bytes(), &line); err != nil {
		t.Fatalf("explain output %q: %v", out.String(), err)
	}
	return string(line.Explain.Value)
}

// explainCode is the exit code of `canon explain` of entry.
func explainCode(t *testing.T, dir string, roots []string, entry string) int {
	t.Helper()
	args := append([]string{"explain", "--project", dir, "--format", "json"}, roots...)
	var out, errs bytes.Buffer
	return cli.Main(context.Background(), append(args, "features.edits:"+entry), cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
}

// editObject is the first line canon edit printed.
type editObject struct {
	Edit struct {
		Applied bool              `json:"applied"`
		Dropped []json.RawMessage `json:"dropped"`
		Undo    json.RawMessage   `json:"undo"`
	} `json:"edit"`
}

// parseEdit decodes the edit object at the head of out.
func parseEdit(t *testing.T, out string) editObject {
	t.Helper()
	first, _, _ := strings.Cut(out, "\n")
	var o editObject
	if err := json.Unmarshal([]byte(first), &o); err != nil {
		t.Fatalf("edit object %q: %v", first, err)
	}
	return o
}

// copyExamples is a copy of examples/ in a new directory.
func copyExamples(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(examplesDir)); err != nil {
		t.Fatal(err)
	}
	return dir
}

// CLI.md §3.15, API.md E22: an undo restores values; the entry's value changes with the edit and is back after the undo.
func TestEditUndoValues(t *testing.T) {
	for name, vr := range valueRequests {
		t.Run(name, func(t *testing.T) {
			dir := copyExamples(t)
			roots := exampleRootsIn(t)
			want := explained(t, dir, roots, vr.entry)
			code, out, errs := editRun(t, dir, roots, vr.request)
			o := parseEdit(t, out)
			if code != 0 || !o.Edit.Applied || errs != "" {
				t.Fatalf("edit: exit %d, %s%s", code, out, errs)
			}
			if vr.dropped && len(o.Edit.Dropped) == 0 {
				t.Errorf("no cascade in %s", out)
			}
			if name == "remove" {
				if got := explainCode(t, dir, roots, vr.entry); got == 0 {
					t.Fatal("the removed entry still explains")
				}
			} else if explained(t, dir, roots, vr.entry) == want {
				t.Fatal("the edit did not change the value")
			}
			if code, out, errs := editRun(t, dir, roots, string(o.Edit.Undo)); code != 0 {
				t.Fatalf("undo: exit %d, %s%s", code, out, errs)
			}
			if got := explained(t, dir, roots, vr.entry); got != want {
				t.Errorf("value differs after undo:\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// CLI.md §3.15: an edit under "editLayer" writes the layer file, and its undo, replayed as printed, restores it and leaves the base file alone.
func TestEditUndoEditLayer(t *testing.T) {
	dir := copyExamples(t)
	roots := exampleRootsIn(t)
	before := snapshot(t, dir)
	code, out, errs := editRun(t, dir, roots, `{"editLayer":"staging","ops":[{"op":"set","path":"features.edits:server.retries","value":3}]}`)
	if code != 0 || errs != "" {
		t.Fatalf("edit: exit %d, %s%s", code, out, errs)
	}
	mid := snapshot(t, dir)
	const base, layer = "features/edits/edits.canon", "features/edits/staging.layer.canon"
	if mid[base] != before[base] || mid[layer] == before[layer] {
		t.Fatalf("the edit wrote the wrong file: base %v, layer %v", mid[base] != before[base], mid[layer] != before[layer])
	}
	undo := parseEdit(t, out).Edit.Undo
	if !strings.Contains(string(undo), `"editLayer":"staging"`) {
		t.Fatalf("undo lacks editLayer: %s", undo)
	}
	if code, out, errs := editRun(t, dir, roots, string(undo)); code != 0 {
		t.Fatalf("undo: exit %d, %s%s", code, out, errs)
	}
	if !equalFiles(before, snapshot(t, dir)) {
		t.Error("files differ after the undo")
	}
}

// CLI.md §3.15: "editLayer" and --edit-layer, given and different, are a usage error; the same is accepted.
func TestEditLayerConflict(t *testing.T) {
	dir := copyExamples(t)
	roots := exampleRootsIn(t)
	request := `{"editLayer":"staging","dryRun":true,"ops":[]}`
	for flagValue, want := range map[string]int{"other": 2, "staging": 0} {
		args := append([]string{"edit", "--project", dir, "--edit-layer", flagValue}, roots...)
		var out, errs bytes.Buffer
		code := cli.Main(context.Background(), args, cli.Env{Stdin: strings.NewReader(request), Stdout: &out, Stderr: &errs, Dir: dir})
		if code != want {
			t.Errorf("--edit-layer %s: exit %d, %s%s", flagValue, code, out.String(), errs.String())
		}
	}
}
