package build

import (
	"context"
	"fmt"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// validOnly refuses an OnlyRoot that is no consumer root, or comes with Adopt (DECISIONS 343).
func (r *run) validOnly(opt BuildOptions) error {
	switch {
	case opt.OnlyRoot == "":
		return nil
	case !r.s.proj.Consumer(opt.OnlyRoot):
		return fmt.Errorf(fmtOnlyRoot, ErrNotConsumerRoot, opt.OnlyRoot)
	case len(opt.Adopt) > 0:
		return fmt.Errorf(fmtOnlyRoot, ErrOnlyRootAdopt, opt.OnlyRoot)
	}
	return nil
}

// consumerOf is the consumer root the display path lies under where project.canon places the
// roots, never this machine, so canon.outputs is the same everywhere; "" for none (DECISIONS 343).
func (r *run) consumerOf(display string) string {
	abs, ok := r.declaredAt(display)
	if !ok {
		return ""
	}
	return check.ConsumerRoot(r.s.proj, abs)
}

// declaredAt is the project-relative path of display where project.canon places the roots.
func (r *run) declaredAt(display string) (string, bool) {
	if r.declared == nil {
		r.declared, _ = project.NewLayout(r.s.proj, declaredAnchor, nil, diag.NewBag(nil, ""))
	}
	at, ok := r.declared.Resolve(display, "", source.Span{}, diag.NewBag(nil, ""))
	return at.Abs, ok
}

// outside is E8028 `outside` for each output of root that this machine places outside root's
// directory, a nested root being placed elsewhere; it reports whether there is none.
func (r *run) outside(root string, kept []*output) bool {
	dir, ok := rootDir(r.s.layout, root), true
	for _, o := range kept {
		if _, in := under(dir, o.Abs); in {
			continue
		}
		abs, _ := r.declaredAt(o.Path) // resolves: its consumer was judged from it
		other := check.OwningRoot(r.s.proj, abs)
		diag.E8028.AtOutside(o.at, root, o.Path, other).Report(r.bags[o.Package])
		ok = false
	}
	return ok
}

// makeRoot lets --only-root write into its root when this machine lacks the root's directory but
// has its parent, which it then creates; with no parent either, E1013 (DECISIONS 343).
func (r *run) makeRoot(outputs []*output, only string) []*output {
	if only == "" || !r.s.layout.Absent(only) {
		return outputs
	}
	dir := rootDir(r.s.layout, only)
	if info, err := r.p.fs.Stat(project.DirOf(dir)); err != nil || !info.IsDir() {
		r.unplaced(only).Report(r.s.own)
		return slices.DeleteFunc(outputs, func(o *output) bool { return o.under == only })
	}
	r.rootMade = dir
	for _, o := range outputs {
		if o.under == only {
			o.under = ""
		}
	}
	return outputs
}

// unplaced is E1013 for root, in the variant of where this machine's path of it is written.
func (r *run) unplaced(root string) *diag.Builder {
	span, written := r.placedAs(root)
	if _, ok := r.p.opt.Roots[root]; ok {
		return diag.E1013.AtOption(span, root, written)
	}
	if r.s.local != nil && slices.ContainsFunc(r.s.local.Roots, func(l project.Root) bool { return l.Name == root }) {
		return diag.E1013.AtLocal(span, root, written)
	}
	return diag.E1013.AtDeclared(span, root, written)
}

// consumers keeps every output but those under a consumer root; with only, those under it and
// the canon.outputs lists, which judge E8028 (DECISIONS 343).
func consumers(outputs []*output, only string) []*output {
	keep := func(o *output) bool { return o.consumer == "" }
	if only != "" {
		keep = func(o *output) bool { return o.consumer == only && !o.remove || projectFile(o) }
	}
	return slices.DeleteFunc(slices.Clone(outputs), func(o *output) bool { return !keep(o) })
}

// projectFile reports a canon.outputs or its removal, which --only-root never writes.
func projectFile(o *output) bool {
	return o.listing || o.remove && path.Base(o.Abs) == outputsName
}

// commitRoot writes or compares the outputs under opt.OnlyRoot alone, nothing after E8028.
func (r *run) commitRoot(ctx context.Context, opt BuildOptions, out *BuildResult, outputs []*output, locks []*lockOut) (*BuildResult, error) {
	kept, ok := r.onlyRoot(opt.OnlyRoot, outputs, locks)
	if !ok {
		res := r.result()
		if err := r.localized(ctx, res); err != nil {
			return nil, err
		}
		out.Result = *res
		return out, nil
	}
	if err := r.commit(opt.Check, out, kept, nil); err != nil {
		return nil, err
	}
	return out, nil
}

// onlyRoot is the outputs under root, and false after an E8028 per lock or list that would change.
func (r *run) onlyRoot(root string, outputs []*output, locks []*lockOut) ([]*output, bool) {
	var kept []*output
	var changed []string
	for _, o := range outputs {
		switch {
		case !projectFile(o):
			kept = append(kept, o)
		case o.remove || o.Status == StatusWritten || o.Status == StatusAdopted:
			changed = append(changed, o.Path)
		}
	}
	for _, l := range locks {
		if l.Status == StatusWritten {
			changed = append(changed, l.Path)
		}
	}
	slices.Sort(changed)
	declared, _ := r.s.proj.Root(root) // found: validOnly accepted root
	for _, p := range slices.Compact(changed) {
		diag.E8028.AtChange(declared.Span, root, p).Report(r.s.own)
	}
	return kept, r.outside(root, kept) && len(changed) == 0
}
