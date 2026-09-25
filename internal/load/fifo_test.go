//go:build !windows

package load_test

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// fifoWaitTimeout bounds TestResolveFileRefusesFIFO: past it, Load tried to read the FIFO.
const fifoWaitTimeout = 5 * time.Second

// WIRE.md §6.1: a single-file form refuses a FIFO before reading it, its E7004 naming ReadNotRegular.
func TestResolveFileRefusesFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "p.txt")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	layout, ok := project.NewLayout(&project.Project{}, dir, nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	l := &load.Loader{FS: project.OS(), Layout: layout, Set: set}
	req := load.Request{Pkg: "p", Bag: bag}
	done := make(chan struct{})
	go func() {
		l.Load(context.Background(), req, textExpr("p.txt"), types.StringType)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(fifoWaitTimeout):
		t.Fatal("Load did not return: it tried to read the FIFO")
	}
	fd := bag.Findings()
	if len(fd) != 1 || fd[0].Code != diag.E7004.Def().Code || !strings.Contains(fd[0].Message, "not a regular file") {
		t.Errorf(`findings = %+v, want one %s naming "not a regular file"`, fd, diag.E7004.Def().Code)
	}
}
