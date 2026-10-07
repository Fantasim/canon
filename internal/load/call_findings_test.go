package load_test

import (
	"bytes"
	"context"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
)

// callFile names the archive's load call text; typeFile its expected type keyword (WIRE.md §6.1).
const (
	callFile = "call"
	typeFile = "type"
)

// loadCall parses "let x = <text>" as one source file, returning the load expression it names.
func loadCall(t *testing.T, text string) *syntax.LoadExpr {
	t.Helper()
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	src, err := set.Add("call.canon", "/call.canon", []byte("package p\n\nlet x = "+text+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, bag)
	if f == nil || len(bag.Findings()) > 0 {
		t.Fatalf("call %q: %v", text, bag.Findings())
	}
	for _, d := range f.Decls {
		if ld, ok := d.(*syntax.LetDecl); ok {
			if le, ok := ld.Value.(*syntax.LoadExpr); ok {
				return le
			}
		}
	}
	t.Fatalf("call %q: not a load expression", text)
	return nil
}

// callType is the expected type a "type" control file names (WIRE.md §6.1's four result shapes).
func callType(kind string) types.Type {
	switch strings.TrimSpace(kind) {
	case "strings":
		return &types.ListType{Elem: &types.ListType{Elem: types.StringType}}
	case "defines":
		return &types.TableType{Elem: types.DefineType}
	case "text":
		return types.StringType
	default:
		return rowType()
	}
}

// TestFindingsCall runs every call.txtar's load call, rendering what load reports (WIRE.md §6).
func TestFindingsCall(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		set := &source.FileSet{}
		bag := diag.NewBag(set, "p")
		data, ok := archiveFile(c.Archive, callFile)
		if !ok {
			t.Skip("no call file: a load.dir pattern case, TestFindings' own")
		}
		text := strings.TrimSuffix(string(data), "\n")
		src, err := set.Add(callFile, path.Join(projectDir, callFile), data)
		if err != nil {
			t.Fatal(err)
		}
		span := source.Span{File: src.ID, Start: 0, End: source.Pos(len(text))}
		kind, _ := archiveFile(c.Archive, typeFile)
		fsys := newMemFS(c.Archive)
		l := &load.Loader{FS: fsys, Layout: machineLayout(t, fsys, bag), Set: set}
		req := load.Request{Pkg: "p", Span: span, Bag: bag}
		_, _, err = l.Load(context.Background(), req, loadCall(t, text), callType(string(kind)))
		if err != nil {
			t.Fatalf("%s: %v (a findings case reports through the bag, never a Go error)", c.Path, err)
		}
		l.FinishDefines() // one call per archive: W7101 is deferred to the build's end (WIRE.md §6.8)
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, set, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}
