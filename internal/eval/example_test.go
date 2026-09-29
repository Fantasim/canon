package eval_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// served is a fixture host, as tests of eval use before load and verify exist: it serves no
// file and finds every value valid.
type served struct{}

func (served) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (served) Verify(context.Context, eval.Root, value.Value) bool { return true }

// A build checks with eval's folder, then forces each top-level value; the refinement bound
// FARM_MAX_MODELS is folded by the same evaluator.
func Example() {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, _ := fs.Add("farm/farm.canon", "/farm/farm.canon", []byte(`/// Farms.
package farm

const FARM_MAX_MODELS = 100

/// How many models a farm may hold.
let maxModels: Int(0..=FARM_MAX_MODELS) = FARM_MAX_MODELS - 1

/// The first squares.
let squares: [Int] = [n * n for n in 1..=4]
`))
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, served{}, bags, eval.Options{})
	for _, name := range []string{"FARM_MAX_MODELS", "maxModels", "squares"} {
		v, _ := ev.Force(ctx, eval.Root{Pkg: "farm", Name: name})
		fmt.Print(name, " = ", v.CanonText(), "; ")
	}
	ev.BeginVerification(ctx)
	fmt.Println(len(bags["farm"].Findings()), "findings")
	// Output: FARM_MAX_MODELS = 100; maxModels = 99; squares = [1, 4, 9, 16]; 0 findings
}

// The origins `canon explain config.server.port --layer louis` prints (CLI.md §3.7).
func ExampleEvaluator_History() {
	ctx := context.Background()
	fs := &source.FileSet{}
	srcs := map[string]string{"studio/studio.canon": `/// Studio.
package studio

/// The server.
record Server {
  /// Listening port.
  port: Int(1024..=65535) = 8765
}

/// Settings.
record Config {
  /// The server.
  server: Server = {}
}

/// The settings.
let config: Config = {}
`, "studio/louis.layer.canon": `package studio
layer louis

amend config {
  server.port: 9000
}
`}
	var files []*syntax.File
	for _, name := range []string{"studio/studio.canon", "studio/louis.layer.canon"} {
		src, _ := fs.Add(name, "/"+name, []byte(srcs[name]))
		files = append(files, syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "")))
	}
	bags, opt := check.Bags{}, eval.Options{Layers: []string{"louis"}}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, opt))
	ev := eval.New(prog, served{}, bags, opt)
	config, _ := ev.Force(ctx, eval.Root{Pkg: "studio", Name: "config"})
	port := config.(*value.Record).Fields[0].(*value.Record).Fields[0]
	for _, v := range ev.History(port) {
		loc := fs.Locate(v.Prov().Span)
		line := fmt.Sprintf("%s %s:%d", v.CanonText(), loc.Path, loc.Line)
		if layer := v.Prov().Layer; layer != "" {
			line += " layer " + layer
		}
		fmt.Println(line)
	}
	// Output:
	// 9000 studio/louis.layer.canon:5 layer louis
	// 8765 studio/studio.canon:7
}

// A conformance vector runs alone on its own step cap; in TS mode an integer outside
// TypeScript's safe range is E8303 where Go computes the value.
func ExampleEvaluator_Vector() {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, _ := fs.Add("shop/shop.canon", "/shop/shop.canon", []byte(`/// Shops.
package shop

/// A price list.
record Price {
  /// The unit price.
  unit: Int

  /// The price of n units.
  export fn times(self, n: Int) -> Int { return unit * n }
}

/// The price of the example.
let price: Price = { unit: 3 }
`))
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, served{}, bags, eval.Options{})
	recv, _ := ev.Force(ctx, eval.Root{Pkg: "shop", Name: "price"})
	times := prog.Info.Defs[files[0].Decls[0].(*syntax.RecordDecl).Body.Items[1].(*syntax.FnDecl).Name]
	c := eval.Call{Fn: times, Recv: recv, Args: []value.Value{&value.Int{V: 1 << 52, T: types.IntType}}}
	for _, ts := range []bool{false, true} {
		o := ev.Vector(ctx, c, eval.VectorMode{Steps: 1_000_000, TS: ts})
		if o.Code != "" {
			fmt.Println("TS", ts, o.Code)
			continue
		}
		fmt.Println("TS", ts, o.Value.CanonText())
	}
	// Output:
	// TS false 13510798882111488
	// TS true E8303
}

// A workspace keeps one memo for its evaluators: the second evaluator replays the entry the
// first evaluated, charging the same steps.
func ExampleEvaluator_UseMemo() {
	ctx := context.Background()
	fs := &source.FileSet{}
	srcs := map[string]string{"zoo/zoo.canon": `/// Zoos.
package zoo

/// An animal.
record Animal {
  /// Legs.
  legs: Int = 4
}

/// The animals.
let animals: table Animal = {}
`, "zoo/owl.canon": `package zoo

entry animals.owl { legs: 1 + 1 }
`}
	var files []*syntax.File
	for _, name := range []string{"zoo/zoo.canon", "zoo/owl.canon"} {
		src, _ := fs.Add(name, "/"+name, []byte(srcs[name]))
		files = append(files, syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "")))
	}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, eval.Options{}))
	memo := eval.NewMemo()
	for range 2 {
		ev := eval.New(prog, served{}, bags, eval.Options{})
		ev.UseMemo(memo, 1)
		v, _ := ev.Force(ctx, eval.Root{Pkg: "zoo", Name: "animals"})
		fmt.Println(v.CanonText())
	}
	// Output:
	// {owl: Animal{legs: 2}}
	// {owl: Animal{legs: 2}}
}
