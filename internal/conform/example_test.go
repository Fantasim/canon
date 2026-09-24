package conform_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// oneShot evaluates each vector in an evaluator of its own, whose budget is the vector's cap,
// and reads the first error its bag captured; it records no test calls.
type oneShot struct {
	prog *check.Program
	fs   *source.FileSet
}

func (oneShot) TestCalls(context.Context, string, []check.Object) []conform.Call { return nil }

func (o oneShot) Evaluate(ctx context.Context, c conform.Call, m conform.Mode) conform.Outcome {
	bags := check.Bags{c.Fn.Pkg(): diag.NewBag(o.fs, c.Fn.Pkg())}
	v, _ := eval.New(o.prog, served{}, bags, eval.Options{Budget: m.Steps}).Call(ctx, c.Fn, c.Recv, c.Args)
	for _, f := range bags[c.Fn.Pkg()].Findings() {
		return conform.Outcome{Code: f.Code, Exceeded: conform.NoLimit}
	}
	return conform.Outcome{Value: v}
}

// A package fn with a refined parameter, in a package with a code emit: its vectors are its
// type range, 0, 1, -1 and each bound's neighbours, and the evaluator says what each gives.
func Example() {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte(`/// A.
package a

/// Half of a level.
export fn half(n: Int(0..=10)) -> Int { return n / 2 }

emit cpp { out: "out/a" }
`))
	if err != nil {
		fmt.Println(err)
		return
	}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	fold := eval.NewFolder(bags, eval.Options{})
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, fold)
	host := stageE{eval.New(prog, served{}, bags, eval.Options{})}
	pkgs := ir.Build(ctx, ir.Input{Program: prog, Project: project.New("demo", project.Version{Minor: 1}), Bags: bags, Host: host, Fold: fold})
	if err := conform.Fill(ctx, prog, pkgs, oneShot{prog: prog, fs: fs}, bags); err != nil {
		fmt.Println(err)
		return
	}
	var line []string
	for _, v := range pkgs[0].Fns[0].Vectors {
		line = append(line, v.Args[0].CanonText()+" -> "+textOf(v.Want)+" "+string(v.Code))
	}
	fmt.Println(strings.Join(line, ", "))
	// Output: -9223372036854775808 -> - E3204, -1 -> - E3204, 0 -> 0 , 1 -> 0 , 9 -> 4 , 10 -> 5 , 11 -> - E3204, 9223372036854775807 -> - E3204
}
