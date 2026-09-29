package cli

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
)

// formatJSONSources normalizes the JSON files the parsed sources load. FORMATTER.md §14.1
func (r *fmtRun) formatJSONSources() error {
	if len(r.sources) == 0 {
		return nil
	}
	p, err := build.Open(r.fsys, r.root, build.Options{Roots: r.inv.opt.roots})
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	found, err := p.JSONSources(r.inv.ctx, r.sources)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	for _, f := range found {
		if err := r.inv.ctx.Err(); err != nil {
			return err
		}
		if err := r.formatJSON(f); err != nil {
			return err
		}
	}
	return nil
}

// formatJSON notes a JSON source's canonical layout; one that does not parse stays. FORMATTER.md §16
func (r *fmtRun) formatJSON(f build.JSONSource) error {
	abs, ok := r.visit(f.Abs)
	if !ok {
		return nil
	}
	data, err := r.read(f.Display, abs)
	if err != nil {
		return err
	}
	src, err := r.set.Add(f.Display, abs, data)
	if err != nil {
		return fmt.Errorf(fmtArgs, f.Display, errTooLarge)
	}
	bag := diag.NewBag(&r.set, "")
	root, err := jsonsrc.Parse(src, bag)
	if err != nil {
		return r.rejectJSON(bag, f.Display, data, err)
	}
	if bytes.Equal(src.Content, f.Content) { // the bytes the loads read: their offsets hold
		canonicalNumbers(root, f.Numbers)
	}
	r.record(f.Display, abs, data, jsonsrc.Format(root))
	return nil
}

// canonicalNumbers sets the text of each number token of numbers, found by its span (DECISIONS 165).
func canonicalNumbers(root *jsonsrc.Node, numbers []build.Number) {
	if len(numbers) == 0 {
		return
	}
	texts := make(map[source.Span]string, len(numbers))
	for _, n := range numbers {
		texts[source.Span{Start: n.Start, End: n.End}] = n.Text
	}
	stack := []*jsonsrc.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], n.Elems...)
		for _, m := range n.Members {
			stack = append(stack, m.Value)
		}
		if text, ok := texts[source.Span{Start: n.Span.Start, End: n.Span.End}]; ok && n.Kind == jsonsrc.Number {
			n.Text = text
		}
	}
}

// rejectJSON keeps the finding of a JSON source that does not parse; only an error jsonsrc does not document is returned.
func (r *fmtRun) rejectJSON(bag *diag.Bag, display string, data []byte, err error) error {
	switch {
	case errors.Is(err, jsonsrc.ErrSyntax), errors.Is(err, jsonsrc.ErrDuplicateKey):
	case errors.Is(err, jsonsrc.ErrEncoding):
		load.ReportEncoding(bag, display, data, err)
	default:
		return fmt.Errorf(fmtWrap, err)
	}
	r.reject(bag)
	return nil
}
