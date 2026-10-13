package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/tools/txtar"
)

// The release test: how many projects it opens one after another, and the bound on the live
// heap after the last against after the first (measured: 1.0 with the release, 4.7 without).
const (
	projectsVisited = 5
	projectsBound   = 2.0
)

// DECISIONS 368, CLI.md §4: projects visited in turn hold one project's memory.
func TestProjectsReleasedOnClose(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	c := startMem(t, base, config{debounce: memDebounce})
	var samples []sample
	for k := range projectsVisited {
		dir := filepath.Join(base, string(rune('a'+k)))
		writeSynth(t, dir, shapeSmall)
		rel := filepath.ToSlash(filepath.Join(filepath.Base(dir), shapeSmall.candidate(0)))
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		must(t, err)
		c.open(rel, string(data))
		c.wait()
		samples = append(samples, measure()) // the project open: what one project holds
		c.close(rel)
		c.wait()
		c.settle()
		t.Logf("project %d open: %v", k+1, samples[k])
	}
	end := measure()
	c.stop()
	t.Logf("all closed: %v", end)
	if ratio := float64(samples[projectsVisited-1].heap) / float64(samples[0].heap); ratio > projectsBound {
		t.Errorf("live heap with the %dth project open is %.2f times that with the first, over %.1f: closed projects are kept",
			projectsVisited, ratio, projectsBound)
	}
}

// DECISIONS 368, CLI.md §4: a release during a pass and a request is quiet, then clears.
func TestReleaseDuringPass(t *testing.T) {
	computing, answering, free := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var held, freed sync.Once
	t.Cleanup(func() { freed.Do(func() { close(free) }) })
	passes := 0
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	done := s.startWith(config{debounce: debounce, computing: func(ctx context.Context) {
		if passes++; passes == 2 {
			close(computing)
			<-ctx.Done()
		}
	}, answering: func() { held.Do(func() { close(answering); <-free }) }})
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	must(t, s.wait(nil))
	must(t, s.change([]string{"proj/a/a.canon", "edits/bool.canon"}))
	waitClosed(t, computing)
	must(t, s.send(map[string]any{"jsonrpc": "2.0", "id": 7, "method": methodHover, "params": positionParamsOf(s, "proj/a/a.canon")}))
	waitClosed(t, answering)
	must(t, s.closeDoc([]string{"proj/a/a.canon"}))
	must(t, s.wait(nil))
	freed.Do(func() { close(free) })
	must(t, s.response([]byte("7")))
	out := string(joined(s.rec.since(0)))
	if !strings.Contains(out, fmt.Sprintf(`"id":7,"error":{"code":%d`, codeModified)) {
		t.Errorf("the request on the released project did not answer ContentModified: %s", out)
	}
	published := s.publications()
	if n := len(published); n != 2 || !strings.Contains(published[1], `"diagnostics":[]`) {
		t.Errorf("publications %q, want the first pass's then the project's findings cleared", published)
	}
	must(t, s.in.Close())
	<-done
	if out := string(joined(s.rec.since(0))); strings.Contains(out, methodLog) {
		t.Errorf("the release was logged: %s", out)
	}
}

// DECISIONS 368: a project released while its first open runs keeps no workspace.
func TestReleaseDuringOpen(t *testing.T) {
	s := newSession(t, txtar.Parse([]byte(passLaw)))
	abs := s.dir + "/proj/a/a.canon"
	read := func() buffers { return buffers{docs: map[string][]byte{abs: s.files["proj/a/a.canon"]}, gen: 1} }
	var p *workspaceProject
	p = newProject(s.dir+"/proj", projectHooks{opening: func() { p.close() }}, read)
	must(t, p.sync(context.Background()))
	if ws, _ := p.current(); ws != nil {
		ws.Close()
		t.Error("the open that finished after the release kept its workspace")
	}
}

// scavengeQuiet is the quiet the scavenge test waits for, short for a test.
const scavengeQuiet = 50 * time.Millisecond

// DECISIONS 368: after a pass and quiet, freed memory goes back to the OS, once per pass.
func TestScavengeWhenQuiet(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	writeSynth(t, base, shapeSmall)
	scavenged := make(chan struct{}, memPasses)
	c := startMem(t, base, config{debounce: memDebounce, quiet: scavengeQuiet, scavenge: func() { scavenged <- struct{}{} }})
	rel := shapeSmall.candidate(0)
	data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
	must(t, err)
	c.open(rel, string(data))
	c.wait()
	waitScavenged(t, scavenged)
	c.change(rel, string(data)+"\n")
	c.wait()
	waitScavenged(t, scavenged)
	time.Sleep(quiet)
	c.stop()
	if n := len(scavenged); n != 0 {
		t.Errorf("%d more scavenges with no pass between", n)
	}
}

func waitScavenged(t *testing.T, scavenged <-chan struct{}) {
	t.Helper()
	select {
	case <-scavenged:
	case <-time.After(patience):
		t.Fatal("no scavenge after a quiet pass")
	}
}
