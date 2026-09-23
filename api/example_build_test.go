package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

func ExampleProject_Evaluate() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Evaluate(context.Background(), canon.EvalRequest{
		Base:  p.Revision(),
		Path:  "teamboard:statuses.open",
		Draft: []canon.Op{canon.Set("teamboard:statuses.open.label", canon.Str("Ouvert"))},
		Lang:  "fr",
	})
	if errors.Is(err, canon.ErrStale) {
		fmt.Println("the sources changed: read again")
		return
	}
	if err != nil {
		return
	}
	fmt.Println(res.Path, res.Title.Value, res.Title.OK, res.Title.Fallback, res.Subtitle.Value, res.Preview)
	for _, line := range res.Show {
		fmt.Println(line.Owner, line.Key, line.Label, line.Text.Value)
	}
	for _, key := range slices.Sorted(maps.Keys(res.Headings)) {
		h := res.Headings[key]
		fmt.Println(key, h.Title.Value, h.Subtitle.Value, h.Preview, h.Retired, len(h.Cells))
	}
	fmt.Println(res.Revision, len(res.When), len(res.Types), len(res.Findings), res.Summary.Errors, len(res.Dropped))
	// Output:
}

func ExampleProject_Watch() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = p.Watch(ctx, func(ev canon.Event) {
		switch ev.Cause {
		case canon.CauseExternal, canon.CauseOverlay:
			fmt.Println(ev.Revision, ev.Files)
		case canon.CauseEdit:
			fmt.Println(ev.Revision, ev.Packages, len(ev.Findings), ev.Summary.Errors, ev.Err)
		}
	})
	if err != nil {
		return
	}
	<-ctx.Done()
	// Output:
}

func ExampleProject_Build() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Build(context.Background(), canon.BuildOptions{
		Packages: []string{"teamboard"},
		Targets:  []canon.Target{canon.TargetGo, canon.TargetCpp, canon.TargetTS, canon.TargetJSON, canon.TargetView},
		Check:    true,
		Adopt:    []string{"@source/_Common/ItemProp.h"},
	})
	if err != nil {
		return
	}
	unchanged := 0
	for _, o := range res.Outputs {
		switch o.Status {
		case canon.OutputWritten, canon.OutputStale, canon.OutputAdopted:
			fmt.Println(o.Status, o.Target, o.Package, o.Path)
		case canon.OutputUnchanged:
			unchanged++
		}
	}
	for _, l := range res.Lock {
		fmt.Println(l.Package, l.File, len(l.Lines))
	}
	fmt.Println(unchanged, res.Stale, res.Check.HasErrors())
	// Output:
}

func ExampleProject_Test() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	res, err := p.Test(context.Background(), canon.TestOptions{Packages: []string{"teamboard"}, Run: "^status"})
	if err != nil {
		return
	}
	for _, t := range res.Tests {
		for _, f := range t.Failures {
			fmt.Printf("%s %s:%d %s\n  expected: %s\n  got: %s\n", t.Name, f.File, f.Line, f.Expect, f.Expected, f.Got)
		}
		fmt.Println(t.Package, t.Name, t.Passed)
	}
	fmt.Println(res.Passed, res.Failed)
	// Output:
}

func ExampleFormat() {
	out, err := canon.Format("teamboard/taxonomy.canon", []byte("package teamboard\nconst VERSION=7\n"))
	var serr *canon.SyntaxError
	if errors.As(err, &serr) && errors.Is(err, canon.ErrSyntax) {
		fmt.Println(len(serr.Findings), "syntax findings")
		return
	}
	if err != nil {
		return
	}
	fmt.Print(string(out))
	// Output:
}

func ExampleFormatJSONSource() {
	out, err := canon.FormatJSONSource([]byte(`{"b":1,"a":[1,2]}`))
	if errors.Is(err, canon.ErrSyntax) {
		fmt.Println("not JSON:", err)
		return
	}
	if err != nil {
		return
	}
	fmt.Print(string(out))
	// Output:
}

func ExampleVersion() {
	v := canon.Version()
	fmt.Println(v.Compiler, v.Languages)
	fmt.Println(v.Fingerprint, v.ViewModel, v.Lock)
	// Output: 0.1.0 [0.1]
	// canon-fp v1 canon-vm/1 canon.lock v1
}
