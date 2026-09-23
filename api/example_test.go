package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	canon "github.com/fantasim/canonlang/api"
)

// openExamples opens the project of the repository's examples/; it fails while the API is a
// stub, and every example then returns before printing anything.
func openExamples() (*canon.Project, error) {
	root, err := canon.FindProject("../examples/teamboard")
	if err != nil {
		return nil, err
	}
	return canon.Open(root, canon.Options{Lang: "fr", Cache: "off"})
}

func ExampleOpen() {
	root, err := canon.FindProject("../examples/teamboard")
	if errors.Is(err, canon.ErrNoProject) {
		fmt.Println("not inside a Canon project")
		return
	}
	if err != nil {
		return
	}
	p, err := canon.Open(root, canon.Options{
		Layers:      []string{"staging"},
		Lang:        "fr",
		Roots:       map[string]string{"resource": "_fixtures/resource"},
		Workers:     1,
		MaxFindings: 100,
		Logger:      slog.New(slog.DiscardHandler),
	})
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
	// Output:
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
		fmt.Println(pkg.Name, pkg.Dir, pkg.Imports, pkg.Layers, len(pkg.Files))
	}
	// Output:
}

func ExampleProject_Check() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Check(context.Background(), "teamboard", "resource...")
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
	res, err := p.Check(context.Background())
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
	describeEditability(v.Editable)
	fmt.Println(v.Type.Expr, string(v.JSON()))
	// Output:
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
	for _, r := range res.Refs {
		switch r.Kind {
		case canon.RefValue, canon.RefKey:
			fmt.Println(r.Kind, r.Package, r.Path)
		case canon.RefCode, canon.RefView, canon.RefCheck, canon.RefLayer:
			fmt.Printf("%s %s:%d\n", r.Kind, r.File, r.Line)
		}
	}
	// Output:
}

func ExampleProject_ViewModel() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	vm, err := p.ViewModel(context.Background(), "teamboard")
	if err != nil {
		return
	}
	var doc map[string]any
	if err := vm.Decode(&doc); err != nil {
		return
	}
	fmt.Println(vm.Package, vm.Revision, len(vm.JSON()), doc["$schema"])
	// Output:
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
	// Output:
}
