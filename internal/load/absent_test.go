package load_test

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

// touchLog is a project.FS that notes every name asked of it.
type touchLog struct {
	project.FS
	mu    sync.Mutex
	names []string
}

func (l *touchLog) note(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.names = append(l.names, name)
}

func (l *touchLog) ReadFile(name string) ([]byte, error) {
	l.note(name)
	return l.FS.ReadFile(name)
}

func (l *touchLog) Stat(name string) (fs.FileInfo, error) {
	l.note(name)
	return l.FS.Stat(name)
}

func (l *touchLog) ReadDir(name string) ([]fs.DirEntry, error) {
	l.note(name)
	return l.FS.ReadDir(name)
}

// absentForms is one load form of each kind, as written, with the root they read.
var absentForms = []struct{ name, call string }{
	{"load", `load("@%s/data/a.json")`},
	{"load at", `load("@%s/data/a.json", at: "rows")`},
	{"load.dir", `load.dir("@%s/data/*.json")`},
	{"load.dir recursive", `load.dir("@%s/**/*.json")`},
	{"load.csv", `load.csv("@%s/data/a.csv")`},
	{"load.text", `load.text("@%s/data/a.txt")`},
	{"load.defines", `load.defines("@%s/Define/a.h")`},
}

// runForm is one load call against a tree holding /here/data/a.json, the log of what it touched
// and the codes it reported.
func runForm(t *testing.T, call string) (touched []string, codes []string, ok bool) {
	t.Helper()
	tree := newMemFS(&txtar.Archive{Files: []txtar.File{
		{Name: "/here/data/a.json", Data: []byte(`[{"label":"x"}]`)},
		{Name: "/here/data/a.csv", Data: []byte("a,b\n")},
		{Name: "/here/data/a.txt", Data: []byte("hi")},
		{Name: "/here/Define/a.h", Data: []byte("#define A 1\n")},
	}})
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	layout := machineLayout(t, tree, bag)
	log := &touchLog{FS: tree}
	l := &load.Loader{FS: log, Layout: layout, Set: set}
	e := loadCall(t, call)
	_, ok, err := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, e, callType(callKindOf(call)))
	if err != nil {
		t.Fatalf("%s: %v", call, err)
	}
	for _, f := range bag.Findings() {
		codes = append(codes, string(f.Code))
	}
	return log.names, codes, ok
}

// callKindOf is the "type" control file a call needs: its expected type's shape.
func callKindOf(call string) string {
	switch {
	case strings.HasPrefix(call, "load.csv"):
		return "strings"
	case strings.HasPrefix(call, "load.text"):
		return "text"
	case strings.HasPrefix(call, "load.defines"):
		return "defines"
	}
	return "rows"
}

// WIRE.md §2.2 rule 5: a load form under an absent optional root reports one finding and touches nothing of it.
func TestAbsentRootReadsNothing(t *testing.T) {
	for _, f := range absentForms {
		t.Run(f.name, func(t *testing.T) {
			touched, codes, ok := runForm(t, strings.Replace(f.call, "%s", goneRoot, 1))
			if ok || len(codes) != 1 || codes[0] != string(diag.E7009.Def().Code) {
				t.Fatalf("ok=%v codes=%v, want exactly the absent-root finding", ok, codes)
			}
			for _, name := range touched {
				if strings.Contains(name, "/"+goneRoot) {
					t.Errorf("the absent root was asked for %s", name)
				}
			}
		})
	}
}

// WIRE.md §2.2 rule 5: an optional root that is present reads as any root does.
func TestPresentOptionalRootReads(t *testing.T) {
	for _, f := range absentForms {
		t.Run(f.name, func(t *testing.T) {
			_, codes, _ := runForm(t, strings.Replace(f.call, "%s", hereRoot, 1))
			for _, c := range codes {
				if c == string(diag.E7009.Def().Code) {
					t.Fatalf("absent-root finding under a present root: %v", codes)
				}
			}
		})
	}
	touched, codes, ok := runForm(t, `load.text("@here/data/a.txt")`)
	if !ok || len(codes) != 0 || len(touched) == 0 {
		t.Fatalf("ok=%v codes=%v touched=%v, want a clean read of the file", ok, codes, touched)
	}
}
