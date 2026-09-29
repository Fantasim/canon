package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/api/vm"
)

// openExamples opens the repository's examples/ project in memory, every root redirected
// (exampleOptions); an Example of a method that is still a stub returns before printing.
func openExamples() (*canon.Project, error) {
	opts := exampleOptions()
	opts.Lang = "fr"
	return canon.Open(exampleRoot, opts)
}

func ExampleFindProject() {
	root, err := canon.FindProject("../examples/teamboard")
	if errors.Is(err, canon.ErrNoProject) {
		fmt.Println("not inside a Canon project")
		return
	}
	if err != nil {
		return
	}
	fmt.Println(filepath.Base(root))
	// Output: examples
}

func ExampleOpen() {
	opts := exampleOptions()
	opts.Layers = []string{"staging"}
	opts.Lang = "fr"
	opts.Workers = 1
	opts.MaxFindings = 100
	opts.Logger = slog.New(slog.DiscardHandler)
	p, err := canon.Open(exampleRoot, opts)
	var perr *canon.ProjectError
	if errors.As(err, &perr) {
		fmt.Println(errors.Is(err, canon.ErrUnsupportedVersion), len(perr.Findings))
		return
	}
	if err != nil {
		return
	}
	defer p.Close()
	fmt.Println(filepath.Base(p.Root()))
	// Output: examples
}

func ExampleProject_Packages() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	pkgs, err := p.Packages(context.Background())
	if err != nil {
		return
	}
	for _, pkg := range pkgs {
		if pkg.Name == "teamboard" || pkg.Name == "balance.parity" {
			fmt.Println(pkg.Name, pkg.Dir, pkg.Imports, pkg.Layers, pkg.Files)
		}
	}
	// Output:
	// balance.parity balance/parity [] [knights] [balance/parity/knights.layer.canon balance/parity/sweep_plan.canon]
	// teamboard teamboard [sovcommon.roles sovcommon.ui] [] [teamboard/taxonomy.canon]
}

func ExampleProject_Check() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	// teamboard and sovcommon... force no `load`; a package that does fails the whole call
	// until load lands (M3, DECISIONS 196).
	res, err := p.Check(context.Background(), "teamboard", "sovcommon...")
	switch {
	case errors.Is(err, canon.ErrUnknownPackage), errors.Is(err, canon.ErrUnknownLayer):
		fmt.Println("usage:", err)
		return
	case errors.Is(err, canon.ErrClosed), err != nil:
		return
	}
	for _, f := range res.Findings {
		if f.Severity == canon.SeverityError || f.Severity == canon.SeverityWarning {
			fmt.Printf("%s[%s] %s:%d:%d %s\n", f.Severity, f.Code, f.File, f.Line, f.Col, f.Message)
		}
	}
	for _, t := range res.Summary.Truncated {
		fmt.Println(t.Package, t.Errors, t.Warnings)
	}
	fmt.Println(res.HasErrors(), res.Summary.Errors, res.Summary.Warnings, res.Summary.Packages)
	// Output:
	// warning[W1701] sovcommon/time/time.canon:6:1 20 texts of package sovcommon.time have no fr translation (canon i18n status sovcommon.time --lang fr --list)
	// false 0 1 4
}

func ExampleProject_LockCheck() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.LockCheck(context.Background())
	if err != nil {
		return
	}
	fmt.Println(res.Revision, res.Packages, res.HasErrors())
	// Output:
}

func ExampleFinding_MarshalJSON() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	// teamboard prints no finding (its own golden); pipeline prints one, its round trip stable.
	res, err := p.Check(context.Background(), "teamboard", "pipeline")
	if err != nil {
		return
	}
	for _, f := range res.Findings {
		line, err := json.Marshal(f)
		if err != nil {
			return
		}
		var back canon.Finding
		if err := json.Unmarshal(line, &back); err != nil {
			return
		}
		fmt.Println(string(line))
	}
	// Output:
	// {"severity":"warning","code":"W1701","file":"pipeline/potion.canon","line":7,"col":1,"endLine":7,"endCol":17,"package":"pipeline","message":"14 texts of package pipeline have no fr translation (canon i18n status pipeline --lang fr --list)"}
}

func ExampleCheckResult_HasErrors() {
	var none *canon.CheckResult
	failed := &canon.CheckResult{Summary: canon.Summary{Errors: 2, Packages: 1}}
	fmt.Println(none.HasErrors(), failed.HasErrors())
	// Output: false true
}

func ExampleProject_Value() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	v, err := p.Value(context.Background(), "teamboard:statuses.open.next[0]")
	var perr *canon.PathError
	switch {
	case errors.Is(err, canon.ErrAmbiguousPath) && errors.As(err, &perr):
		fmt.Println("qualify the path:", perr.Candidates)
		return
	case errors.Is(err, canon.ErrNoValue) && errors.As(err, &perr):
		fmt.Println(len(perr.Findings), "findings explain why")
		return
	case errors.Is(err, canon.ErrNoPath), errors.Is(err, canon.ErrBadPath), err != nil:
		return
	}
	describe(v)
	describeOrigin(v.Origin)
	fmt.Println(v.Type.Expr, string(v.JSON()), string(v.Type.VM))
	// Output: teamboard:statuses.open.next[0] taken
	// literal at teamboard/taxonomy.canon:172
	// ref teamboard.statuses "taken" {"kind":"ref","collection":"teamboard:statuses","element":"teamboard.Status","keyType":"string","count":6,"active":6}
}

func ExampleEditability() {
	opts := exampleOptions()
	opts.Layers = []string{"louis"}
	p, err := canon.Open(exampleRoot, opts)
	if err != nil {
		return
	}
	defer p.Close()
	for _, path := range []string{"config.server.host", "config.server.port"} {
		if v, err := p.Value(context.Background(), path); err == nil {
			describeEditability(v.Editable)
		}
	}
	// Output: edits service/resourcestudio/resourcestudio.canon
	// set by layer louis
}

func ExampleValue_String() {
	v := &canon.Value{Path: "teamboard:statuses.open.by", Kind: canon.KindNone, Text: "none"}
	fmt.Println(v, v.IsNone())
	// Output: none true
}

func ExampleProject_Refs() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Refs(context.Background(), "teamboard:statuses.open")
	if errors.Is(err, canon.ErrBadOp) {
		fmt.Println("not an entry, element or member")
		return
	}
	if err != nil {
		return
	}
	var where []string
	for _, r := range res.Refs {
		switch r.Kind {
		case canon.RefValue, canon.RefKey:
			where = append(where, r.Package+":"+r.Path)
		case canon.RefCode, canon.RefView, canon.RefCheck, canon.RefLayer:
			where = append(where, fmt.Sprintf("%s:%d", r.File, r.Line))
		}
	}
	fmt.Println(res.Target, len(where))
	fmt.Println(where[0])
	// Output:
	// teamboard:statuses.open 6
	// teamboard:statuses.taken.next[0]
}

func ExampleProject_ViewModel() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	model, err := p.ViewModel(context.Background(), "teamboard")
	if errors.Is(err, canon.ErrUnknownPackage) {
		fmt.Println("no such package")
		return
	}
	if err != nil {
		return
	}
	var doc vm.ViewModel
	if err := model.Decode(&doc); err != nil {
		return
	}
	fmt.Println(model.Package, doc.Schema, doc.Language, model.Revision == p.Revision())
	fmt.Println(doc.Types["teamboard.Status"].Kind, doc.Values["teamboard:statuses"].Type.Kind)
	// Output:
	// teamboard canon-vm/1 0.1 true
	// record table
}

func ExampleProject_SetOverlay() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	before := p.Revision()
	if err := p.SetOverlay("teamboard/taxonomy.canon", []byte("package teamboard\n")); err != nil {
		return
	}
	defer func() { _ = p.ClearOverlay("teamboard/taxonomy.canon") }()
	fmt.Println(before != p.Revision())
	// Output: true
}

func ExampleWriteFindings() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Check(context.Background(), "teamboard")
	if err != nil {
		return
	}
	opts := canon.WriteOptions{Summary: res.Summary, Duration: res.Duration, Golden: true}
	if err := canon.WriteFindings(os.Stdout, res.Findings, opts); err != nil {
		fmt.Println(err)
	}
	// Output: 0 errors, 0 warnings in 1 package (…)
}
