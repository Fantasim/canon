package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

var (
	flagMem      = flag.Int("lsp.mem", 0, "edits of the memory bench on the large synthetic project (0: skip)")
	flagMemProf  = flag.String("lsp.memprof", "", "directory the memory bench writes its heap profiles to")
	flagMemDir   = flag.String("lsp.memdir", "", "a project directory the memory bench copies into a temporary directory and edits there, instead of the synthetic one: roots outside it need absolute paths (project.local.canon)")
	flagMemFiles = flag.String("lsp.memfiles", "", "the files of -lsp.memdir it opens and edits, comma-separated")
	flagMemPace  = flag.Bool("lsp.mempace", false, "type at a person's pace, passes cancelled, instead of waiting for each pass")
)

// The memory session's rhythm: a sample every memBatch edits, a hover, definition and references
// every memAsk, a buffer closed and another opened every memSwap, a save every memSave.
const (
	memBatch    = 50
	memAsk      = 3
	memSwap     = 25
	memSave     = 40
	memDebounce = 10 * time.Millisecond
	memPasses   = 1024
	memLiveHeap = "/gc/heap/live:bytes"
	memRSS      = "VmRSS:"
	memPeak     = "VmHWM:"
	memKiB      = 1024
	memMiB      = 1 << 20
)

// sample is the memory held: the resident set as the process holds it, its peak so far, then
// after a GC the live heap and the resident set once the freed pages went back to the OS
// (the sets on Linux; 0 elsewhere).
type sample struct {
	held, peak, heap, rss uint64
}

func measure() sample {
	held, peak := residentSet(memRSS), residentSet(memPeak)
	runtime.GC()
	debug.FreeOSMemory()
	m := []metrics.Sample{{Name: memLiveHeap}}
	metrics.Read(m)
	return sample{held: held, peak: peak, heap: m[0].Value.Uint64(), rss: residentSet(memRSS)}
}

// residentSet is the process's /proc status line key in bytes, 0 when /proc cannot tell.
func residentSet(key string) uint64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for line := range strings.Lines(string(data)) {
		if rest, ok := strings.CutPrefix(line, key); ok {
			kib, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
			if err == nil {
				return kib * memKiB
			}
		}
	}
	return 0
}

func (m sample) String() string {
	return fmt.Sprintf("rss held %.0f MiB (peak %.0f), live heap %.1f MiB, rss scavenged %.0f MiB",
		float64(m.held)/memMiB, float64(m.peak)/memMiB, float64(m.heap)/memMiB, float64(m.rss)/memMiB)
}

// memClient drives a server as an editor does, keeping nothing of what it receives but which
// responses came, so that its own memory stays flat.
type memClient struct {
	t      testing.TB
	dir    string
	in     *io.PipeWriter
	out    *ackWriter
	passes chan struct{}
	done   <-chan error
	next   int

	mu        sync.Mutex
	published int // the passes published, and how many had when the server last scavenged
	scavenged int
}

func startMem(t testing.TB, dir string, cfg config) *memClient {
	inR, inW := io.Pipe()
	c := &memClient{t: t, dir: filepath.ToSlash(dir), in: inW, out: newAckWriter(), passes: make(chan struct{}, memPasses)}
	cfg.passed = func() {
		c.mu.Lock()
		c.published++
		c.mu.Unlock()
		c.passes <- struct{}{}
	}
	if scavenge := cfg.scavenge; scavenge != nil {
		cfg.scavenge = func() {
			scavenge()
			c.mu.Lock()
			c.scavenged = c.published
			c.mu.Unlock()
		}
	}
	done := make(chan error, 1)
	safego.Go(func() error { return serve(context.Background(), inR, c.out, cfg) }, func(err error) { done <- err })
	c.done = done
	c.request(methodInitialize, map[string]any{"processId": nil, "rootUri": nil, "capabilities": map[string]any{}})
	return c
}

func (c *memClient) send(v map[string]any) {
	v["jsonrpc"] = jsonrpcV2
	body, err := json.Marshal(v)
	if err == nil {
		_, err = fmt.Fprintf(c.in, headerFormat+"%s", len(body), body)
	}
	if err != nil {
		c.t.Fatal(err)
	}
}

func (c *memClient) note(method string, params any) {
	c.send(map[string]any{"method": method, "params": params})
}

// request sends a request and waits for its response.
func (c *memClient) request(method string, params any) {
	c.next++
	id := c.next
	c.send(map[string]any{"id": id, "method": method, "params": params})
	if err := c.out.await(id); err != nil {
		c.t.Fatal(err)
	}
}

func (c *memClient) uri(rel string) string { return uriOf(c.dir) + "/" + rel }

func (c *memClient) open(rel, text string) {
	c.note(methodDidOpen, map[string]any{"textDocument": map[string]any{"uri": c.uri(rel), "languageId": "canon", "version": 1, "text": text}})
}

func (c *memClient) change(rel, text string) {
	c.note(methodDidChange, map[string]any{"textDocument": map[string]any{"uri": c.uri(rel), "version": 2}, "contentChanges": []any{map[string]any{"text": text}}})
}

func (c *memClient) close(rel string) {
	c.note(methodDidClose, map[string]any{"textDocument": map[string]any{"uri": c.uri(rel)}})
}

// ask sends hover, definition and references at line, character of rel, as a reader does.
func (c *memClient) ask(rel string, line, char int) {
	at := map[string]any{"textDocument": map[string]any{"uri": c.uri(rel)}, "position": map[string]any{"line": line, "character": char}}
	c.request(methodHover, at)
	c.request(methodDefinition, at)
	refs := map[string]any{"textDocument": at["textDocument"], "position": at["position"], "context": map[string]any{"includeDeclaration": true}}
	c.request(methodReferences, refs)
}

// wait blocks until a pass published.
func (c *memClient) wait() {
	select {
	case <-c.passes:
	case <-time.After(patience):
		c.t.Fatalf("no pass in %v", patience)
	}
}

// settle waits for the passes still due, then for quiet.
func (c *memClient) settle() {
	for {
		select {
		case <-c.passes:
		case <-time.After(quiet):
			return
		}
	}
}

// awaitScavenge waits until the server scavenged after its last pass published.
func (c *memClient) awaitScavenge() {
	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) {
		time.Sleep(quiet)
		c.mu.Lock()
		done := c.scavenged == c.published
		c.mu.Unlock()
		if done {
			return
		}
	}
	c.t.Fatalf("no scavenge after the last pass in %v", patience)
}

func (c *memClient) stop() {
	c.request(methodShutdown, nil)
	c.note(methodExit, nil)
	if err := <-c.done; err != nil {
		c.t.Fatal(err)
	}
}

// ackWriter is a server's output read as an editor does, remembering only the ids answered.
type ackWriter struct {
	mu   sync.Mutex
	cond *sync.Cond
	ids  map[int]bool
}

func newAckWriter() *ackWriter {
	w := &ackWriter{ids: map[int]bool{}}
	w.cond = sync.NewCond(&w.mu)
	return w
}

func (w *ackWriter) Write(p []byte) (int, error) {
	body, err := readFrame(bufio.NewReader(bytes.NewReader(p)))
	if err != nil {
		return 0, err
	}
	var r struct {
		ID *int `json:"id"`
	}
	if json.Unmarshal(body, &r) == nil && r.ID != nil {
		w.mu.Lock()
		w.ids[*r.ID] = true
		w.cond.Broadcast()
		w.mu.Unlock()
	}
	return len(p), nil
}

func (w *ackWriter) await(id int) error {
	deadline := time.AfterFunc(patience, func() { w.mu.Lock(); w.cond.Broadcast(); w.mu.Unlock() })
	defer deadline.Stop()
	start := time.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	for !w.ids[id] {
		if time.Since(start) > patience {
			return fmt.Errorf("no response to %d in %v", id, patience)
		}
		w.cond.Wait()
	}
	delete(w.ids, id)
	return nil
}

// writeProfile writes the heap profile, after a GC, to dir/name; nothing for dir "".
func writeProfile(t testing.TB, dir, name string) {
	if dir == "" {
		return
	}
	runtime.GC()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		t.Fatal(err)
	}
}
