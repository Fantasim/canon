package workspace

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"
	"testing"
)

var errStatPanic = errors.New("a stat that panics")

// API.md S1, DECISIONS 421: new bytes keeping the size and modification time are missed by a
// refresh, and read once suspected, as one external event; the same bytes keep the snapshot; a
// name added under a listed directory whose stamp is unchanged is listed once suspected.
func TestSuspectRereadsKeptStamp(t *testing.T) {
	fsys := newKeptFS()
	p := keptProject(t, fsys)
	var events []Event
	defer p.Subscribe(func(e Event) { events = append(events, e) })()
	fsys.replace(t)
	if got, _ := readKept(t, p); got != memoPackage {
		t.Fatalf("the stamp showed the change: %q", got)
	}
	p.Suspect(keptFile)
	got, s := readKept(t, p)
	if got != keptChanged || len(events) != 1 || events[0].Cause != CauseExternal || !slices.Equal(events[0].Files, []string{keptDisplay}) {
		t.Fatalf("API.md S1: a suspect read %q, events %+v", got, events)
	}
	p.Suspect(keptFile)
	if _, again := readKept(t, p); again != s || len(events) != 1 {
		t.Errorf("API.md S1: the same bytes made a new snapshot, events %+v", events)
	}
	added := keptDir + "/z.canon"
	if err := fsys.WriteFile(added, []byte(memoPackage)); err != nil {
		t.Fatal(err)
	}
	if listedKept(t, p, "z.canon") {
		t.Fatal("the directory's stamp showed the name added")
	}
	p.Suspect(added)
	if !listedKept(t, p, "z.canon") {
		t.Error("API.md S1: a suspect's directory was not listed again")
	}
}

// API.md S1, DECISIONS 421: a refresh cancelled or failed after taking the suspects keeps them,
// so the next one still reads the file again.
func TestSuspectKeptOnFailedRefresh(t *testing.T) {
	for _, c := range []struct {
		name string
		fail func(cancel context.CancelFunc)
	}{
		{"cancelled", func(cancel context.CancelFunc) { cancel() }},
		{"failed", func(context.CancelFunc) { panic(errStatPanic) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			fsys := newKeptFS()
			p := keptProject(t, fsys)
			fsys.replace(t)
			p.Suspect(keptFile)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var once sync.Once
			fsys.setOnStat(func(name string) {
				if name == keptFile {
					once.Do(func() { c.fail(cancel) })
				}
			})
			if _, err := p.Read(ctx); err == nil {
				t.Fatal("the refresh did not fail")
			}
			if got, _ := readKept(t, p); got != keptChanged {
				t.Errorf("API.md S1: the suspect was lost: the next refresh reads %q", got)
			}
		})
	}
}

// listedKept reports file in x/'s listing as the snapshot a Read returns holds it.
func listedKept(t *testing.T, p *Project, file string) bool {
	t.Helper()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.fs.ReadDir(keptDir)
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(list, func(e fs.DirEntry) bool { return e.Name() == file })
}
