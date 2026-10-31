package lsp

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/must"
	"golang.org/x/tools/txtar"
)

// watchedLaw is a project whose loaded file is in error, then fixed keeping its size.
const watchedLaw = `-- proj/project.canon --
project acme {
  canon: "0.1"
}
-- proj/a/a.canon --
/// A.
package a

/// Counts.
let counts: [Int] = load("counts.json")
-- proj/a/counts.json --
[1, "x", 3]
-- fixed.json --
[1, 222, 3]
`

// API.md S1, IMPLEMENTATION-PLAN §8.4 Sync, DECISIONS 421.
func TestWatchedFilesRereadKeptStamp(t *testing.T) {
	reports := func(uri string) func(*session) any {
		return func(s *session) any {
			return map[string]any{"changes": []any{map[string]any{"uri": strings.ReplaceAll(uri, "$URI", s.uri), "type": 2}}}
		}
	}
	cases := []struct {
		name   string
		params func(s *session) any // the notification's
		fixed  bool                 // the diagnostics follow the change
		logs   int                  // the window/logMessage notifications it gives
	}{
		{name: "reported", params: reports("$URI/proj/a/counts.json"), fixed: true},
		{name: "no file of a project", params: reports("$URI/elsewhere/counts.json")},
		{name: "not a file", params: reports("untitled:counts.json")},
		{name: "undecodable, still recomputed", params: func(*session) any { return map[string]any{"changes": 5} }, logs: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSession(t, txtar.Parse([]byte(watchedLaw)))
			counts := filepath.Join(s.dir, "proj", "a", "counts.json")
			kept := time.Date(2020, time.January, 2, 3, 4, 5, 6, time.UTC)
			must.NoError(t, os.Chtimes(counts, kept, kept))
			done := s.startWith(config{debounce: debounce, base: keptStampFS{build.OS()}})
			must.NoError(t, s.initialize(nil))
			must.NoError(t, s.open([]string{"proj/a/a.canon"}))
			must.NoError(t, s.wait(nil))
			countsURI := s.uri + "/proj/a/counts.json"
			if n := s.lastDiagnostics(countsURI); n == 0 {
				t.Fatalf("counts.json: no diagnostics before the fix")
			}
			must.NoError(t, os.WriteFile(counts, s.files["fixed.json"], 0o644))
			must.NoError(t, os.Chtimes(counts, kept, kept))
			must.NoError(t, s.note(methodWatched, tc.params(s)))
			must.NoError(t, s.wait(nil)) // a pass published: the projects were recomputed
			if n := s.lastDiagnostics(countsURI); (n == 0) != tc.fixed {
				t.Errorf("counts.json after the notification: %d diagnostics, the fix seen = %v", n, tc.fixed)
			}
			must.NoError(t, s.request(map[string]any{"jsonrpc": jsonrpcV2, "id": 1, "method": methodShutdown})) // answered after the note's log
			if logs := s.logged(); logs != tc.logs {
				t.Errorf("%d messages logged, want %d", logs, tc.logs)
			}
			must.NoError(t, s.note(methodExit, nil))
			must.NoError(t, <-done)
		})
	}
}

// lastDiagnostics is how many diagnostics the last publication for uri carried, 0 for none.
func (s *session) lastDiagnostics(uri string) int {
	n := 0
	for _, msg := range s.rec.since(0) {
		var m struct {
			Method string
			Params struct {
				URI         string
				Diagnostics []json.RawMessage
			}
		}
		if json.Unmarshal(msg, &m) == nil && m.Method == methodPublish && m.Params.URI == uri {
			n = len(m.Params.Diagnostics)
		}
	}
	return n
}

// logged is how many window/logMessage notifications the server sent.
func (s *session) logged() int {
	n := 0
	for _, msg := range s.rec.since(0) {
		var m struct{ Method string }
		if json.Unmarshal(msg, &m) == nil && m.Method == methodLog {
			n++
		}
	}
	return n
}

// keptStampFS is the disk with Windows' stamp: a stat carries no change time and no file
// identity (DECISIONS 374).
type keptStampFS struct {
	build.WriteFS
}

func (f keptStampFS) Stat(name string) (fs.FileInfo, error) {
	info, err := f.WriteFS.Stat(name)
	if err != nil {
		return nil, err
	}
	return keptStampInfo{info}, nil
}

// EvalSymlinks keeps the disk's links followed (WIRE.md §6.5).
func (f keptStampFS) EvalSymlinks(name string) (string, error) {
	return project.EvalSymlinks(f.WriteFS, name)
}

type keptStampInfo struct {
	fs.FileInfo
}

func (keptStampInfo) Sys() any { return nil }
