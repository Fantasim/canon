package edit

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// undoCheck is an edit's Undo being verified in memory against the edit's base (API.md E22 over
// E23's order); where it misses, the value inverses inside the region concerned give way to its
// restore, the entry-level ones stay (log-2026-09-29 U-E22-r).
type undoCheck struct {
	a         *applier
	after     *Snapshot       // the edit's result
	over      *overlayFS      // the files as the edit leaves them
	roots     []Path          // the roots whose values are compared
	plain     []Operation     // the inverses E23 lists
	dropped   []bool          // the plain inverses refused, left out
	regions   []string        // the items restored, in path order, none inside another
	fieldwise map[string]bool // the regions restored field by field
	places    map[string]int  // how many places each region's restore may be tried at
	tried     map[string]int  // the place each region's restore is tried at
	origin    []int           // for each operation of the Undo built: its plain index, or -1 - its region's
	last      *Snapshot       // the state the last verification left
	inError   []string        // the values the base holds in error, which the Undo need not give back
}

// verifiedUndo is ops, E23's inverses, if they restore every value (E22), else undoCheck's
// verified Undo. Every edit under an edit layer is verified; off layers, only several operations
// touching a dependent field or a driver, or with a cascade: a single Set never pays (NFR-01).
func (a *applier) verifiedUndo(ops []Operation, touched []string) ([]Operation, error) {
	needed := a.env.EditLayer != "" || a.multi && (a.dependent || len(a.cascadeUndo) > 0 || a.facts != nil && len(a.facts.ids) > 0)
	if !needed || len(ops) == 0 {
		return ops, nil
	}
	if err := a.settle(); err != nil {
		return nil, err
	}
	c := &undoCheck{a: a, after: a.snap, over: a.overlay(), plain: ops, dropped: make([]bool, len(ops)), fieldwise: map[string]bool{}, places: map[string]int{}, tried: map[string]int{}}
	c.roots = a.comparedRoots(ops, touched)
	if c.unreadable() {
		return ops, nil // a root the edit leaves unreadable takes no inverse at all (E1): left to the re-check
	}
	c.inError = c.heldInError()
	if err := a.overLocks(c.over); err != nil {
		return nil, err
	}
	if a.env.EditLayer != "" {
		return c.layerUndo()
	}
	return c.regionUndo()
}

// regionUndo is the Undo outside an edit layer: E23's inverses, repaired region by region until
// a dry run gives every value back (E22).
func (c *undoCheck) regionUndo() ([]Operation, error) {
	cur := c.build()
	for range maxUndoRounds {
		failed, err := c.run(cur)
		if err != nil {
			return nil, err
		}
		misses := c.misses()
		done := len(failed) == 0 && len(misses) == 0
		if done && !c.lockBroken() {
			return cur, nil
		}
		if done {
			break // a lock fact the Undo would undo, which no restore repairs (E23)
		}
		if !c.repair(cur, failed, misses) {
			break
		}
		cur = c.build()
	}
	return nil, fmt.Errorf(fmtWrapped, ErrInternal, errUndoUnverified)
}

// comparedRoots are the roots an edit's operations, its inverses and its cascades name, in path
// order; with a Rename, which rewrites references anywhere, every let and const of the packages
// it writes (E11).
func (a *applier) comparedRoots(ops []Operation, touched []string) []Path {
	set := map[string]Path{}
	add := func(p Path) {
		set[Path{Package: p.Package, Root: p.Root}.String()] = Path{Package: p.Package, Root: p.Root}
	}
	for _, r := range a.named {
		add(Path{Package: r.pkg.Path, Root: r.obj.Name()})
	}
	for _, op := range ops {
		if p, err := Parse(op.Path); err == nil && p.Package != "" {
			add(p)
		}
	}
	if a.renamed {
		a.base.eachRoot(touched, add)
	}
	out := make([]Path, 0, len(set))
	for _, k := range slices.Sorted(maps.Keys(set)) {
		out = append(out, set[k])
	}
	return out
}

// unreadable reports a compared root whose value the base computes and the edit's result does not.
func (c *undoCheck) unreadable() bool {
	return slices.ContainsFunc(c.roots, func(r Path) bool {
		_, before := forced(c.a.base, r)
		_, after := forced(c.after, r)
		return before && !after
	})
}

// run applies ops in memory to the edit's result as Apply does, each one refused skipped with
// its writes taken back, then their cascades (E15); nothing is written. It returns the refused
// operations by index, with why; the state they leave is c.last.
func (c *undoCheck) run(ops []Operation) (map[int]error, error) {
	env := c.a.env
	env.Project = env.Project.Over(c.over)
	b := newApplier(c.a.ctx, env, c.after)
	failed := map[int]error{}
	for i, op := range ops {
		b.step = i
		cp := b.checkpoint()
		if err := b.operation(op); err != nil {
			if fatal(err) {
				return nil, err
			}
			b.restore(cp)
			failed[i] = err
		}
	}
	b.step = cascadeStep
	if err := b.cascade(); err != nil && fatal(err) {
		return nil, err
	}
	if err := b.settle(); err != nil {
		return nil, err
	}
	c.last = b.snap
	return failed, nil
}

// fatal reports an error no inverse causes: the file system's, or a cancellation.
func fatal(err error) bool {
	var io *ioError
	return errors.As(err, &io) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// misses are the canonical paths where the state the last run left differs from the base, an
// entry's file included, but those inside a value the base holds in error.
func (c *undoCheck) misses() []string {
	d := &valueDiff{ordered: c.ordered, placed: c.samePlace}
	for _, r := range c.roots {
		x, before := forced(c.a.base, r)
		y, now := forced(c.last, r)
		switch {
		case before && now:
			d.value(r.String(), x, y)
		case before != now:
			d.out = append(d.out, r.String())
		}
	}
	return slices.DeleteFunc(d.out, c.excluded)
}

// excluded reports path at or inside a value the base holds in error, or one the Undo leaves as
// the edit's result holds it, a lock fact (API.md E22, E23).
func (c *undoCheck) excluded(path string) bool {
	inside := func(e string) bool { return within(path, e, pathMarks, true) }
	return slices.ContainsFunc(c.inError, inside) || c.a.facts.kept(path)
}

// heldInError are the paths of the compared roots' values the base holds in a dependent
// mismatch (E3802) not held as its JSON source wrote it (E15, DECISIONS 175): the Undo leaves
// them as the cascade drops them (log-2026-10-01 M4.1 review rulings).
func (c *undoCheck) heldInError() []string {
	var pkgs, out []string
	for _, r := range c.roots {
		pkgs = append(pkgs, r.Package)
	}
	for _, pkg := range slices.Compact(slices.Sorted(slices.Values(pkgs))) {
		bag := c.a.base.a.Bag(pkg)
		if bag == nil {
			continue
		}
		for _, f := range bag.Findings() {
			if path := pkg + packageMark + f.Path; f.Code == diag.E3802.Def().Code && f.Path != "" && !c.heldAt(path) {
				out = append(out, path)
			}
		}
	}
	return out
}

// heldAt reports the base's value at path a decoded symbol as its JSON source wrote it.
func (c *undoCheck) heldAt(path string) bool {
	p, err := Parse(path)
	if err != nil {
		return false
	}
	res, err := c.a.base.open(p)
	return err == nil && decodedIn(res.Target)
}

// samePlace reports entry y of the last run, at path, where entry x of the base was: in a file of
// x's path, or for an entry the Undo recreates, of a path its template can give for its key,
// whatever the templated fields (E22, N1, N2, N3, N4; log-2026-10-01 M4.1 rulings).
func (c *undoCheck) samePlace(path string, x, y *value.Record) bool {
	if x.P == nil || y.P == nil {
		return x.P == y.P
	}
	got := c.last.display(y.P.Span.File)
	if placed, ok := c.recreatedAt(path, y); ok {
		return placed(got)
	}
	return c.a.base.display(x.P.Span.File) == got
}

// recreatedAt tells the files the Undo may recreate entry y at path in: the request removed it
// (and maybe added it back), or the edit's result lacks it, and no Rename gives it back its name;
// false otherwise (E22; log-2026-10-01 M4.1 ruling a).
func (c *undoCheck) recreatedAt(path string, y *value.Record) (func(string) bool, bool) {
	p, err := Parse(path)
	if err != nil || len(p.Segs) != 1 || !c.recreates(path, p) {
		return nil, false
	}
	res, err := c.last.open(Path{Package: p.Package, Root: p.Root})
	if err != nil {
		return nil, false
	}
	let, ok := res.root.obj.Decl().(*syntax.LetDecl)
	if !ok {
		return nil, false
	}
	key := y.Ident.Key.Text()
	if tpl, isTpl := filesTemplate(let); isTpl {
		return templateMatch(packageDir(res.root.pkg), tpl, key)
	}
	if c.ordered(Path{Package: p.Package, Root: p.Root}.String()) {
		return nil, false // a literal's entry comes back in its literal
	}
	load, _ := syntax.Unparen(let.Value).(*syntax.LoadExpr)
	display, err := (&opCtx{a: c.a, res: res}).entryPath(let, load, y, &value.Str{V: key, T: types.StringType})
	return func(got string) bool { return got == display }, err == nil
}

// templateMatch tells the display paths an @files template can give the entry key in the package
// directory dir: `{id}` its key, each other `{f}` any one path segment (N2, N3).
func templateMatch(dir, tpl, key string) (func(string) bool, bool) {
	var b strings.Builder
	b.WriteString(regexp.QuoteMeta(dir + pathSep))
	for {
		open := strings.IndexByte(tpl, tplOpen)
		if open < 0 {
			b.WriteString(regexp.QuoteMeta(tpl))
			break
		}
		end := strings.IndexByte(tpl[open:], tplClose)
		if end < 0 {
			return nil, false
		}
		b.WriteString(regexp.QuoteMeta(tpl[:open]))
		if tpl[open+1:open+end] == pseudoID {
			b.WriteString(regexp.QuoteMeta(key))
		} else {
			b.WriteString(templateSegment)
		}
		tpl = tpl[open+end+1:]
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	return func(got string) bool {
		at := re.FindStringIndex(got)
		return at != nil && at[0] == 0 && at[1] == len(got)
	}, true
}

// ordered reports the table or map at path ordered by its source, which a Move restores:
// not one whose entries their files' paths order (E22, N1).
func (c *undoCheck) ordered(path string) bool {
	p, err := Parse(path)
	if err != nil {
		return true
	}
	res, err := c.a.base.open(p)
	if err != nil {
		return true
	}
	return c.a.base.judge(res, OpMove, c.a.env.EditLayer).last().ordersLiteral(cursor{})
}

// forced is the value of root r in s, false when s has no such root or cannot compute it.
func forced(s *Snapshot, r Path) (value.Value, bool) {
	root, err := s.lookup(r)
	if err != nil || root.enum != nil {
		return nil, false
	}
	v, err := s.force(root)
	return v, err == nil
}

// eachRoot calls f on every let and const of the packages pkgs names.
func (s *Snapshot) eachRoot(pkgs []string, f func(Path)) {
	for _, pkg := range s.pkgs {
		if !slices.Contains(pkgs, pkg.Path) {
			continue
		}
		for _, obj := range pkg.Decls {
			if obj.Kind() == check.ObjLet || obj.Kind() == check.ObjConst {
				f(Path{Package: pkg.Path, Root: obj.Name()})
			}
		}
	}
}

// recreates reports the Undo recreating the entry at path, p parsed: an AddEntry of it among E23's
// inverses (a Remove's), or the edit's result lacking it, and no Rename giving it back its name.
func (c *undoCheck) recreates(path string, p Path) bool {
	renamed := slices.ContainsFunc(c.plain, func(op Operation) bool {
		to, ok := renamedTo(op)
		return ok && to == path
	})
	added := slices.ContainsFunc(c.plain, func(op Operation) bool {
		k, isKey := op.Key.(PathKey)
		return op.Kind == OpAddEntry && isKey && childPath(op.Path, entrySeg(value.Key{S: string(k)})) == path
	})
	_, err := c.after.open(p)
	return !renamed && (added || err != nil)
}
