package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// editing is an editor session: what it opens next, and its open documents, oldest first.
type editing struct {
	c    *memClient
	pick func(k int) string // the k-th document opened
	open []string
	text map[string]string
	seen int  // candidates opened so far
	pace bool // typed at a person's pace, passes cancelled, rather than waiting for each
	step int  // the pauses made so far
}

// pauses are a typist's, cycled: keystrokes inside the debounce, then a pause long enough for a
// pass to start, which the next keystroke cancels when the check is slower.
var pauses = []time.Duration{60 * time.Millisecond, 60 * time.Millisecond, 60 * time.Millisecond, 400 * time.Millisecond}

// next waits for the pass a change starts, or, paced, for the next keystroke.
func (e *editing) next() {
	if !e.pace {
		e.c.wait()
		return
	}
	time.Sleep(pauses[e.step%len(pauses)])
	e.step++
}

// candidate is the k-th document an editor opens: the first big file, then files spread over
// packages and files.
func (sh shape) candidate(k int) string {
	const pkgStride, fileStride = 7, 13
	if k == 0 && sh.big > 0 {
		return fmt.Sprintf(synthPkg+"/"+synthBig+".canon", 0)
	}
	return fmt.Sprintf(synthPkg+"/"+synthFile+".canon", k*pkgStride%sh.packages, k*fileStride%sh.files)
}

// sessionRun is how an editing session runs: its edits, the directory its heap profiles go to
// (none for ""), and whether it types at a person's pace.
type sessionRun struct {
	edits int
	prof  string
	pace  bool
}

// editSession runs r against the project at dir, sampling the memory after the first full check
// and every memBatch edits. Paced, the server runs as Serve runs it, and each sample waits for
// its scavenge first.
func editSession(t testing.TB, dir string, pick func(int) string, r sessionRun) []sample {
	const opened = 4
	cfg := config{debounce: memDebounce}
	if r.pace {
		cfg = config{debounce: debounce, quiet: scavengeAfter, scavenge: debug.FreeOSMemory}
	}
	e := &editing{c: startMem(t, dir, cfg), pick: pick, text: map[string]string{}, pace: r.pace}
	for range opened {
		e.openNext()
	}
	e.c.wait()
	samples := []sample{e.sample()}
	t.Logf("after the first full check: %v", samples[0])
	writeProfile(t, r.prof, "heap-first.pprof")
	for i := range r.edits {
		e.edit(i)
		if (i+1)%memBatch == 0 {
			samples = append(samples, e.sample())
			t.Logf("after %d edits: %v", i+1, samples[len(samples)-1])
		}
	}
	e.c.settle()
	writeProfile(t, r.prof, "heap-last.pprof")
	e.c.stop()
	return samples
}

// sample is the memory once the passes due published, and, paced, the server scavenged.
func (e *editing) sample() sample {
	e.c.settle()
	if e.pace {
		e.c.awaitScavenge()
	}
	return measure()
}

func (e *editing) openNext() {
	rel := e.pick(e.seen)
	e.seen++
	data, err := os.ReadFile(filepath.Join(e.c.dir, filepath.FromSlash(rel)))
	if err != nil {
		e.c.t.Fatal(err)
	}
	e.text[rel] = string(data)
	e.open = append(e.open, rel)
	e.c.open(rel, string(data))
}

// typing is what the i-th edit puts in place of a row's first `level: `, as typing passes
// through: a new value, a type error, a syntax error, a comment; each text never seen before.
var typing = []func(i int) string{
	func(i int) string { return fmt.Sprintf("level: %d // edit %d: ", 1+i%synthLevels, i) },
	func(i int) string { return fmt.Sprintf("level: \"x%d\" // ", i) },
	func(i int) string { return fmt.Sprintf("level: : %d ", i) },
	func(i int) string { return fmt.Sprintf("// typed %d\n    level: ", i) },
}

// appending is typing for any source: a line appended at its end.
var appending = []func(i int) string{
	func(i int) string { return fmt.Sprintf("\n// edit %d\n", i) },
	func(i int) string { return fmt.Sprintf("\nlet typed%d: Int = \"x\"\n", i) },
	func(i int) string { return fmt.Sprintf("\nlet typed%d = \n", i) },
	func(i int) string { return fmt.Sprintf("\nlet typed%d: Int = %d\n", i, i) },
}

// typed is rel's text after the i-th edit: a row's level changed, else a line appended.
func (e *editing) typed(rel string, i int) string {
	text := e.text[rel]
	if strings.Contains(text, synthLevel) {
		return strings.Replace(text, synthLevel, typing[i%len(typing)](i), 1)
	}
	return text + appending[i%len(appending)](i)
}

// edit is the i-th edit: a new text of an open document as typing makes it; a question, a swap
// of documents or a save on the way.
func (e *editing) edit(i int) {
	rel := e.open[i%len(e.open)]
	text := e.typed(rel, i)
	e.c.change(rel, text)
	e.next()
	if i%memAsk == 0 {
		const line, char = 7, 13
		e.c.ask(rel, line, char)
	}
	if i%memSave == 0 {
		e.save(rel, text)
	}
	if (i+1)%memSwap == 0 {
		e.c.close(e.open[0])
		e.open = e.open[1:]
		e.openNext()
		e.next()
	}
}

// save writes rel's text to the disk and tells the server, as an editor's save and watcher do.
func (e *editing) save(rel, text string) {
	if err := os.WriteFile(filepath.Join(e.c.dir, filepath.FromSlash(rel)), []byte(text), synthPerm); err != nil {
		e.c.t.Fatal(err)
	}
	e.text[rel] = text
	e.c.note(methodWatched, map[string]any{"changes": []any{map[string]any{"uri": e.c.uri(rel), "type": 2}}})
	e.next()
}

// The always-on edit test: its edits, and the bound on the live heap after them against after
// the first full check (measured: 1.06 at 100 edits, 1.19 at 300).
const (
	boundedEdits = 100
	editsBound   = 1.5
)

// DECISIONS 368: an editing session holds about what the first full check held.
func TestEditsBounded(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	writeSynth(t, dir, shapeSmall)
	samples := editSession(t, dir, shapeSmall.candidate, sessionRun{edits: boundedEdits})
	first, last := samples[0], samples[len(samples)-1]
	if ratio := float64(last.heap) / float64(first.heap); ratio > editsBound {
		t.Errorf("live heap after %d edits is %.2f times that after the first check, over %.1f", boundedEdits, ratio, editsBound)
	}
}

// IMPLEMENTATION-PLAN §6 M5, DECISIONS 368: opt-in, `go test ./internal/lsp -run TestMemoryBench -lsp.mem 500`.
func TestMemoryBench(t *testing.T) {
	if *flagMem <= 0 {
		t.Skip("opt-in: -lsp.mem <edits>")
	}
	files := strings.Split(*flagMemFiles, ",")
	pick := func(k int) string { return files[k%len(files)] }
	dir, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	if *flagMemDir != "" {
		must(t, os.CopyFS(dir, os.DirFS(*flagMemDir))) // the copy alone is edited
	} else {
		writeSynth(t, dir, shapeLarge)
		pick = shapeLarge.candidate
	}
	samples := editSession(t, dir, pick, sessionRun{edits: *flagMem, prof: *flagMemProf, pace: *flagMemPace})
	first, last := samples[0], samples[len(samples)-1]
	t.Logf("first full check: %v; after %d edits: %v; ratio heap %.2f rss %.2f", first, *flagMem, last,
		float64(last.heap)/float64(first.heap), float64(last.rss)/float64(first.rss))
}
