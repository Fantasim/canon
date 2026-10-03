package workspace_test

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

const (
	pinnedOutputs = 40
	olderInUse    = 6
	oldOutput     = "old\n"
	heldPinReads  = 3 // under overlays: the writer's pin, the held older snapshot's own, the refresh after
)

// outputName is the i-th file a build writes.
func outputName(i int) string { return fmt.Sprintf("/law/out/f%d.json", i) }

// outputsFS is the law project with pinnedOutputs files a build replaces.
func outputsFS() *memFS {
	files := lawFiles()
	for i := range pinnedOutputs {
		files[outputName(i)] = oldOutput
	}
	return newMemFS(files)
}

// editB is a write that changes b/b.canon, the n-th time.
func editB(n int) func(context.Context, *workspace.Snapshot) error {
	return func(_ context.Context, s *workspace.Snapshot) error {
		return s.Build().FS().(writeFS).WriteFile("/law/b/b.canon", fmt.Appendf(nil, "%s// %d\n", srcB, n))
	}
}

// writeOutputs replaces every output as a build does, returning how many reads it cost.
func writeOutputs(t *testing.T, p *workspace.Project, fsys *memFS) int {
	t.Helper()
	before := fsys.readCount(pinnedOutputs)
	if _, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
		w := s.Build().FS().(writeFS)
		for i := range pinnedOutputs {
			if err := w.WriteFile(outputName(i), []byte("new\n")); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return fsys.readCount(pinnedOutputs) - before
}

// readsOld fails unless every older snapshot reads every output as it was before the write.
func readsOld(t *testing.T, older []*workspace.Snapshot) {
	t.Helper()
	for j, s := range older {
		for i := range pinnedOutputs {
			if data, err := s.Build().FS().ReadFile(outputName(i)); err != nil || string(data) != oldOutput {
				t.Fatalf("older snapshot %d reads %s as %q, %v", j, outputName(i), data, err)
			}
		}
	}
}

// API.md S9, log-2026-09-29 M4 "U3 re-review": a writer replacing many files while older
// snapshots are in use reads each file once, whatever their number, and every older snapshot
// still reads each file as it was before the write.
func TestWritePinsEachFileOnce(t *testing.T) {
	fsys := outputsFS()
	p := open(t, fsys)
	var older []*workspace.Snapshot
	for i := range olderInUse {
		s, err := p.Write(context.Background(), workspace.CauseEdit, editB(i))
		if err != nil {
			t.Fatal(err)
		}
		older = append(older, s)
	}
	if n := writeOutputs(t, p, fsys); n > 2*pinnedOutputs { // the pin, then the refresh after the write
		t.Errorf("the write read its %d files %d times under %d older snapshots", pinnedOutputs, n, olderInUse)
	}
	readsOld(t, older)
}

// API.md S9, §3.4 (log-2026-09-29 M4 U8-r): under overlays a held older snapshot pins its own reads.
func TestWritePinsUnderOverlays(t *testing.T) {
	fsys := outputsFS()
	p := open(t, fsys)
	if err := p.SetOverlay("/law/a/a.canon", []byte(srcA+"// overlay\n")); err != nil {
		t.Fatal(err)
	}
	older := read(t, p)
	next, err := p.Write(context.Background(), workspace.CauseEdit, editB(0))
	if err != nil {
		t.Fatal(err)
	}
	if next == older {
		t.Fatal("the edit published no new snapshot: the held one is the writer's, not an older one")
	}
	runtime.GC() // only the snapshots held are older ones a writer pins for
	if n := writeOutputs(t, p, fsys); n < heldPinReads*pinnedOutputs {
		t.Errorf("under overlays the write read its %d files only %d times: the older snapshot shared its reads", pinnedOutputs, n)
	}
	readsOld(t, []*workspace.Snapshot{older})
}

// syncFS is memFS syncing directories, which it records.
type syncFS struct {
	*memFS
	mu     sync.Mutex
	synced []string
}

func (s *syncFS) SyncDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synced = append(s.synced, dir)
	return nil
}

// API.md §10.3 (log-2026-09-29 M4 U8-r): a snapshot's file system forwards SyncDir, as Chmod.
func TestSnapshotForwardsSyncDir(t *testing.T) {
	fsys := &syncFS{memFS: newMemFS(lawFiles())}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	syncer, ok := read(t, p).Build().FS().(build.DirSyncer)
	if !ok {
		t.Fatal("the snapshot's file system has no SyncDir")
	}
	if err := syncer.SyncDir("/law/b"); err != nil || !slices.Equal(fsys.synced, []string{"/law/b"}) {
		t.Errorf("SyncDir: %v, synced %v", err, fsys.synced)
	}
}

// readCount is how many times the first n outputs were read from the file system.
func (m *memFS) readCount(n int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for i := range n {
		total += m.reads[outputName(i)]
	}
	return total
}
