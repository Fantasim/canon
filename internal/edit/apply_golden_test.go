package edit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/edit"
)

var update = flag.Bool("update", false, "rewrite the edit goldens under testdata/edits instead of comparing")

// The sections of an edit golden: options, edit, files before; then the result and files after.
const (
	secOptions = "options"
	secEdit    = "edit.json"
	secResult  = "result.json"
	afterDir   = "after/"
)

// goldenCase is one golden: the project before, the edit, and how the project is opened.
type goldenCase struct {
	comment   []byte
	options   []byte
	edit      []byte
	before    []txtar.File
	fsys      mapFS
	pkgs      []string
	layers    []string
	editLayer string
	oneLine   bool     // API.md M6: a one-value Set of a scalar changes exactly one line
	crlf      []string // files the test writes with CRLF line ends, which a txtar cannot hold

	undoUnordered bool // E22: entries whose order their files' paths give are compared as a set
}

// API.md E1-E16, E22, E23, M1-M9, N1-N8, IMPLEMENTATION-PLAN 7.4: every edit golden
// applies, writes what its archive holds, and keeps the minimal-write invariant M6.
func TestEditGoldens(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "edits", "*.txtar"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no edit goldens: %v", err)
	}
	for _, p := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(p), ".txtar"), func(t *testing.T) { runGolden(t, p) })
	}
}

// goldenRules are the rules this package's edit goldens prove, each cited as `API.md <id>`.
var goldenRules = []string{
	"E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9", "E10", "E11", "E12", "E13", "E14", "E15",
	"E16", "E22", "E23", "M1", "M2", "M3", "M4", "M5", "M6", "M7", "M8", "M9",
	"N1", "N2", "N3", "N4", "N5", "N6", "N7", "N8", "W5", "W7", "W8", "W9", "W11", "W11a",
}

// IMPLEMENTATION-PLAN.md 7.4: each rule goldenRules lists is cited, as `API.md <id>`, by the
// comment of an edit golden that proves it.
func TestEditGoldensCiteRules(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "edits", "*.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	var comments []byte
	for _, p := range paths {
		ar, err := txtar.ParseFile(p)
		if err != nil {
			t.Fatal(err)
		}
		comments = append(comments, ar.Comment...)
	}
	for _, id := range goldenRules {
		if !regexp.MustCompile(`API\.md ` + id + `\b`).Match(comments) {
			t.Errorf("no edit golden cites API.md %s", id)
		}
	}
}

func runGolden(t *testing.T, path string) {
	ar, err := txtar.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := readCase(t, ar)
	var req struct {
		Ops []edit.Operation `json:"ops"`
	}
	if err := json.Unmarshal(c.edit, &req); err != nil {
		t.Fatalf("edit.json: %v", err)
	}
	s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
	if err == nil {
		checkPlan(t, c, plan)
		checkUndo(t, c, req.Ops, plan)
	}
	got := c.archive(t, plan, err)
	if *update {
		if err := os.WriteFile(path, txtar.Format(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if want, _ := os.ReadFile(path); !bytes.Equal(txtar.Format(got), want) {
		t.Errorf("%s differs from what Apply writes (run with -update and read the diff):\n%s", path, txtar.Format(got))
	}
}

// readCase reads the options, the edit and the files before; generated sections are dropped.
func readCase(t *testing.T, ar *txtar.Archive) goldenCase {
	c := goldenCase{comment: ar.Comment, fsys: mapFS{}}
	for _, f := range ar.Files {
		switch {
		case f.Name == secOptions:
			c.options = f.Data
			c.readOptions(t, f.Data)
		case f.Name == secEdit:
			c.edit = f.Data
		case f.Name == secResult || strings.HasPrefix(f.Name, afterDir):
		default:
			c.before = append(c.before, f)
			c.fsys["law/"+f.Name] = file(string(f.Data))
		}
	}
	for _, name := range c.crlf {
		if f, ok := c.fsys["law/"+name]; ok {
			f.Data = bytes.ReplaceAll(f.Data, []byte("\n"), []byte("\r\n"))
		}
	}
	return c
}

// readOptions reads `key: value` lines: pkgs, layers (space-separated), editLayer, oneLine.
func (c *goldenCase) readOptions(t *testing.T, data []byte) {
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, val, ok := strings.Cut(line, ":")
		val = strings.TrimSpace(val)
		switch {
		case !ok:
			t.Fatalf("options: %q", line)
		case key == "pkgs":
			c.pkgs = strings.Fields(val)
		case key == "layers":
			c.layers = strings.Fields(val)
		case key == "editLayer":
			c.editLayer = val
		case key == "oneLine":
			c.oneLine = val == "true"
		case key == "crlf":
			c.crlf = strings.Fields(val)
		case key == "undoUnordered":
			c.undoUnordered = val == "true"
		}
	}
}

// result is the golden's test-only JSON projection of Apply's result, without Revision
// (log-2026-09-29 M4: EditResult has no JSON form).
type result struct {
	Error        string            `json:"error,omitempty"`
	Changes      []changeJSON      `json:"changes,omitempty"`
	Dropped      []droppedJSON     `json:"dropped,omitempty"`
	Undo         []json.RawMessage `json:"undo,omitempty"`
	Touched      []string          `json:"touched,omitempty"`
	NotCanonical []string          `json:"notCanonical,omitempty"`
}

type droppedJSON struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

type changeJSON struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
}

var changeKinds = map[edit.ChangeKind]string{
	edit.ChangeModified: "modified", edit.ChangeCreated: "created", edit.ChangeDeleted: "deleted",
	edit.ChangeRenamed: "renamed", edit.ChangeRemovedDir: "dirRemoved",
}

// archive is the golden as Apply's result writes it.
func (c goldenCase) archive(t *testing.T, plan *edit.Plan, applyErr error) *txtar.Archive {
	ar := &txtar.Archive{Comment: c.comment}
	if c.options != nil {
		ar.Files = append(ar.Files, txtar.File{Name: secOptions, Data: c.options})
	}
	ar.Files = append(ar.Files, txtar.File{Name: secEdit, Data: c.edit})
	ar.Files = append(ar.Files, c.before...)
	var r result
	if applyErr != nil {
		r.Error = errorText(applyErr)
	} else {
		r = projection(t, plan)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	ar.Files = append(ar.Files, txtar.File{Name: secResult, Data: append(data, '\n')})
	if plan != nil {
		for _, ch := range plan.Changes {
			if ch.After != nil {
				ar.Files = append(ar.Files, txtar.File{Name: afterDir + ch.Path, Data: ch.After})
			}
		}
	}
	return ar
}

func projection(t *testing.T, plan *edit.Plan) result {
	r := result{Touched: plan.Touched, NotCanonical: plan.NotCanonical}
	for _, d := range plan.Dropped {
		r.Dropped = append(r.Dropped, droppedJSON{Path: d.Path, Value: d.Value})
	}
	for _, ch := range plan.Changes {
		r.Changes = append(r.Changes, changeJSON{Kind: changeKinds[ch.Kind], Path: ch.Path, OldPath: ch.OldPath})
	}
	for _, op := range plan.Undo {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("undo %#v: %v", op, err)
		}
		r.Undo = append(r.Undo, b)
	}
	return r
}

// errorText is a refusal as the golden records it: the operation, the sentinel, the reason.
func errorText(err error) string {
	var oe *edit.OpError
	if !errors.As(err, &oe) {
		return err.Error()
	}
	text := fmt.Sprintf("op %d: %v", oe.Index, oe.Err)
	var ne *edit.NotEditableError
	if errors.As(err, &ne) && len(ne.Refs) > 0 {
		text += " refs " + strings.Join(ne.Refs, ", ")
	}
	return text
}

// checkPlan checks N12 and M6 on every file the plan writes, with the before bytes Apply read.
func checkPlan(t *testing.T, c goldenCase, plan *edit.Plan) {
	t.Helper()
	before := map[string][]byte{}
	for name, f := range c.fsys { //canon:unordered fills a map by name
		before[strings.TrimPrefix(name, "law/")] = f.Data
	}
	for _, ch := range plan.Changes {
		old := ch.Path
		if ch.OldPath != "" {
			old = ch.OldPath
		}
		if ch.Before != nil && !bytes.Equal(ch.Before, before[old]) {
			t.Errorf("%s: Before is not the bytes read (API.md N9)", ch.Path)
		}
		checkWritten(t, plan, ch, c.oneLine && ch.Kind == edit.ChangeModified)
	}
	if !slices.ContainsFunc(plan.Changes, func(ch edit.Change) bool { return ch.Kind == edit.ChangeModified }) && c.oneLine {
		t.Errorf("oneLine case modified no file")
	}
}

// checkWritten checks N12 and M6 on a change of any kind: After a fixed point, and each write
// of the file inside the items it rewrote (a normalization of M9 is a write of the whole file).
func checkWritten(t *testing.T, plan *edit.Plan, ch edit.Change, oneLine bool) {
	t.Helper()
	if ch.Kind == edit.ChangeRemovedDir {
		return
	}
	if ch.After != nil {
		checkN12(t, ch.Path, ch.After)
		checkFixedPoint(t, ch.Path, ch.After)
	}
	checkSteps(t, plan, ch, oneLine)
}

// checkN12 is API.md N12: `\n` line ends, exactly one final `\n`.
func checkN12(t *testing.T, path string, b []byte) {
	t.Helper()
	if bytes.ContainsRune(b, '\r') || !bytes.HasSuffix(b, []byte("\n")) || bytes.HasSuffix(b, []byte("\n\n")) {
		t.Errorf("%s: After is not N12-clean: %q", path, b)
	}
}
