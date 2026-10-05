package verify_test

import (
	"bytes"
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

// orderFile lists `pkg.name` roots a case forces first, in order, so stage B verifies them first (EVALUATION.md §5).
const orderFile = "order.txt"

// orderedCase checks, evaluates and verifies a case's packages on one goroutine, its order.txt roots first.
func orderedCase(fx *fixture) {
	fx.t.Helper()
	fx.out = orderedBuild(fx.t, fx.archive)
}

func orderedBuild(t *testing.T, a *txtar.Archive) []byte {
	t.Helper()
	fs := &source.FileSet{}
	parsed := diag.NewBag(fs, "")
	var files []*syntax.File
	for _, f := range a.Files {
		if path.Ext(f.Name) == ".canon" && f.Name != projectFile {
			src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, syntax.Parse(src, syntax.FileSource, parsed))
		}
	}
	ctx, bags := context.Background(), check.Bags{}
	fold := eval.NewFolder(bags, eval.Options{})
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, fold)
	h := &orderedHost{t: t}
	h.ev = eval.New(prog, h, bags, eval.Options{})
	h.ev.UseFolder(fold)
	h.v = verify.New(h.ev, prog, bags, nil)
	for _, name := range strings.Fields(archiveText(a, orderFile)) {
		dot := strings.LastIndex(name, ".")
		if dot < 0 {
			t.Fatalf("order.txt: %q is not pkg.name", name)
		}
		h.ev.Force(ctx, eval.Root{Pkg: name[:dot], Name: name[dot+1:]})
	}
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjLet || obj.Kind() == check.ObjConst {
				h.ev.Force(ctx, eval.Root{Pkg: pkg.Path, Name: obj.Name()})
			}
		}
	}
	h.ev.BeginVerification(ctx)
	return renderBags(t, fs, parsed, bags)
}

// orderedHost is the eval.Host of orderedBuild: stage B is verify's; it loads nothing.
type orderedHost struct {
	t  *testing.T
	ev *eval.Evaluator
	v  *verify.Verifier
}

func (h *orderedHost) Verify(ctx context.Context, root eval.Root, v value.Value) bool {
	res, err := h.v.Check(ctx, root, v)
	if err != nil {
		h.t.Fatal(err)
	}
	if res.Poisoned {
		h.ev.Poison(root)
	}
	for _, u := range res.Unbound {
		h.ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

func (h *orderedHost) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	h.t.Fatal("an ordered case loads nothing")
	return nil, false
}

// archiveText is the archive's file named name, "" for none.
func archiveText(a *txtar.Archive, name string) string {
	for _, f := range a.Files {
		if f.Name == name {
			return string(f.Data)
		}
	}
	return ""
}

// renderBags is the parse findings and every package's, packages by path, in the golden form.
func renderBags(t *testing.T, fs *source.FileSet, parsed *diag.Bag, bags check.Bags) []byte {
	t.Helper()
	all, sum := parsed.Findings(), parsed.Summary()
	sum.Packages = 0
	names := make([]string, 0, len(bags))
	for name := range bags {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		all = append(all, bags[name].Findings()...)
		sum = sum.Merge(bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
