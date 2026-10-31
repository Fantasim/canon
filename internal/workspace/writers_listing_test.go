package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/writelock"
)

// listingRule is the rule TestEditLocksListingReaders proves.
const listingRule = "DECISIONS 417, CLI.md §3.15, API.md E17: "

// listedFiles are package a, whose items are JSON files under a/items/, and c, listing that
// directory for the names starting with p alone: c reads no file an edit there writes or removes.
var listedFiles = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": "/// A.\npackage a\n\n/// Item.\nrecord Item {\n  /// V.\n  v: Int = 1\n}\n\n" +
		"/// Items.\n@files(\"items/{id}.json\")\nlet items: table Item = load.dir(\"items/*.json\")\n",
	"a/items/pan.json": "{\n  \"v\": 2\n}\n",
	"a/items/x.json":   "{\n  \"v\": 3\n}\n",
	"c/c.canon":        "/// C.\npackage c\n\n/// J.\nrecord J {\n  /// V.\n  v: Int\n}\n\n/// Js.\nlet js: [J] = load.dir(\"../a/items/p*.json\")\n",
}

// CLI.md §3.15, DECISIONS 417: an edit adding or removing a name c lists waits for c's lock.
func TestEditLocksListingReaders(t *testing.T) {
	cases := []struct {
		name string
		op   edit.Operation
		file string
		gone bool
	}{
		{"created", edit.Operation{Kind: edit.OpAddEntry, Path: "a:items", Key: edit.Key("pot"), Value: edit.Source("{ v: 4 }")}, "a/items/pot.json", false},
		{"removed", edit.Operation{Kind: edit.OpRemove, Path: "a:items.x"}, "a/items/x.json", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, p := onDisk(t, listedFiles)
			held := holdWriters(t, dir, "c")
			waits, errs := make(chan writelock.Wait, 1), make(chan error, 1)
			req := workspace.EditRequest{Changes: workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{c.op}}, Normalize: true}
			go func() {
				_, err := p.Edit(waitsOn(context.Background(), waits), req)
				errs <- err
			}()
			if w := waitedOrEnded(t, waits, errs); w.Package != "c" {
				t.Fatalf("%sthe edit waited for %+v, want c", listingRule, w)
			}
			if err := held.Release(); err != nil {
				t.Fatal(err)
			}
			if err := <-errs; err != nil {
				t.Fatalf("%sthe edit: %v", listingRule, err)
			}
			_, err := os.Stat(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(c.file)))
			if gone := errors.Is(err, os.ErrNotExist); gone != c.gone {
				t.Errorf("%s%s after the edit: %v, want gone %v", listingRule, c.file, err, c.gone)
			}
		})
	}
}

// waitedOrEnded is the next wait told on waits; an edit ending first, on errs, fails the test with its error.
func waitedOrEnded(t *testing.T, waits <-chan writelock.Wait, errs <-chan error) writelock.Wait {
	t.Helper()
	select {
	case w := <-waits:
		return w
	case err := <-errs:
		t.Fatalf("%sthe edit ended without waiting: %v", listingRule, err)
	case <-time.After(patience):
		t.Fatalf("%sno writer waited", listingRule)
	}
	return writelock.Wait{}
}
