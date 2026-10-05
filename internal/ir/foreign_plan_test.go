package ir_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestForeignPlans is CODEGEN.md §2.8, §3.3, §3.5, §5.8, §5.9, §5.14 and DECISIONS 323: for each go, cpp and ts emit of a case, what the name plans give gen/go, gen/cpp and gen/ts to build other packages' values: the emit's imports with their targets (transitive ones included), the classes it reads and writes, its rows, readers, rt imports, make hooks and define tables, then the plan's problems; stage E's findings follow.
func TestForeignPlans(t *testing.T) {
	golden.Run(t, "testdata/foreign/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != planFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		var b strings.Builder
		for _, p := range w.build(t) {
			for _, e := range p.Emits {
				dumpForeign(&b, p, e)
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

// dumpForeign writes one code emit's foreign plan.
func dumpForeign(b *strings.Builder, p *ir.Package, e *ir.Emit) {
	if e.Target != ir.TargetGo && e.Target != ir.TargetCpp && e.Target != ir.TargetTS {
		return
	}
	fmt.Fprintf(b, "== %s %s %s\n", targetNames[e.Target], p.Name, modeNames[e.Mode])
	var imports []string
	for _, ref := range p.Imports {
		var ts []string
		for _, ie := range ref.Emits {
			ts = append(ts, targetNames[ie.Target])
		}
		imports = append(imports, ref.Name+"["+strings.Join(ts, ",")+"]")
	}
	fmt.Fprintf(b, "  imports: %s\n", strings.Join(imports, " "))
	switch e.Target {
	case ir.TargetGo:
		dumpGoForeign(b, p, ir.PlanGoNames(p, e))
	case ir.TargetCpp:
		dumpCppForeign(b, p, e, ir.PlanCppNames(p, e))
	default:
		names := ir.TSModuleNames(p, e)
		var aliases []string
		for _, imp := range ir.TSImports(p) {
			aliases = append(aliases, fmt.Sprintf("%s=%s(reserved %v)", imp.Pkg, imp.Alias, names[imp.Alias]))
		}
		fmt.Fprintf(b, "  namespaces: %s\n", strings.Join(aliases, " "))
		fmt.Fprintf(b, "  readers: %s\n", strings.Join(withPrefix(names, "read"), " "))
		fmt.Fprintf(b, "  CanonRow reserved: %v\n", names["CanonRow"])
	}
	fmt.Fprintf(b, "  define tables: %s (%d loaded)\n", strings.Join(ir.EmitDefineRefs(p, e), " "), len(ir.EmitDefines(p, e)))
}

// dumpTables writes the tables of other packages' classes the emit builds, each with the hook building its rows.
func dumpTables(b *strings.Builder, u *ir.ForeignUse, entry, row func(*ir.Record) string) {
	for _, t := range u.Tables {
		hook := row(t.Record)
		if t.Entry() {
			hook = entry(t.Record)
		}
		fmt.Fprintf(b, "  table of %s held by %s: %s\n", t.Record.QName(), t.Holder, hook)
	}
}

// dumpEntries writes the entry hooks scope declares for p's records.
func dumpEntries(b *strings.Builder, p *ir.Package, scope map[string]bool, entry func(*ir.Record) string) {
	for _, t := range p.Types {
		if rec, ok := t.(*ir.Record); ok && scope[entry(rec)] {
			fmt.Fprintf(b, "  entry hook %s\n", entry(rec))
		}
	}
}

// withPrefix are the names that start with prefix, sorted.
func withPrefix(names map[string]bool, prefix string) []string {
	var out []string
	for _, n := range slices.Sorted(maps.Keys(names)) {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

// dumpGoForeign writes a go emit's uses, rows, readers, rt imports and hooks, its own and those it calls, then its problems.
func dumpGoForeign(b *strings.Builder, p *ir.Package, pl *ir.GoNamePlan) {
	u := pl.Foreign()
	fmt.Fprintf(b, "  read: %s\n  written: %s\n", classList(u, u.Read), classList(u, u.Written))
	for _, row := range pl.Rows() {
		r := pl.Row(row.Record)
		fmt.Fprintf(b, "  row %s id %s hook %s of %s, held by %s, forwards %s\n", r.Name, r.ID, pl.RowHook(row.Record), row.Record.QName(), row.Origin, goForwarded(r))
	}
	var readers []string
	for _, c := range u.Read {
		readers = append(readers, pl.ReaderName(c))
	}
	fmt.Fprintf(b, "  readers: %s\n", strings.Join(readers, " "))
	for _, imp := range pl.RTImports() {
		fmt.Fprintf(b, "  rt import %s %q of %s\n", imp.Name, imp.Path, imp.Pkg)
	}
	for _, c := range append(ownClasses(p), u.Built()...) {
		for _, h := range goHooks(pl, u, c) {
			fmt.Fprintf(b, "  hook %s\n", h)
		}
	}
	dumpEntries(b, p, ir.GoScopeNames(pl)["package"], pl.EntryHook)
	dumpTables(b, u, pl.EntryHook, pl.RowHook)
	dumpProblems(b, pl.Problems())
}

// dumpCppForeign is dumpGoForeign for a cpp emit, with its make struct and free loaders.
func dumpCppForeign(b *strings.Builder, p *ir.Package, e *ir.Emit, pl *ir.CppNamePlan) {
	u := pl.Foreign()
	fmt.Fprintf(b, "  read: %s\n  written: %s\n  make struct: %s\n", classList(u, u.Read), classList(u, u.Written), pl.MakeStruct(p.Name))
	for _, row := range pl.Rows() {
		fmt.Fprintf(b, "  row %s hook %s of %s, held by %s, scope %s\n", pl.RowName(row.Record), pl.RowHook(row.Record), row.Record.QName(), row.Origin, strings.Join(slices.Sorted(maps.Keys(ir.CppScopeNames(pl)[pl.RowName(row.Record)])), " "))
	}
	for _, v := range p.Values {
		if l := pl.ForeignLoader(v); l != "" {
			fmt.Fprintf(b, "  free loader %s of %s\n", l, v.Name)
		}
	}
	if e.Mode != ir.ModeBaked {
		var readers []string
		for _, c := range u.Read {
			readers = append(readers, pl.ReaderName(c))
		}
		fmt.Fprintf(b, "  readers: %s\n", strings.Join(readers, " "))
	}
	for _, c := range append(ownClasses(p), u.Built()...) {
		for _, h := range cppHooks(pl, u, c) {
			fmt.Fprintf(b, "  hook %s\n", h)
		}
	}
	dumpEntries(b, p, ir.CppScopeNames(pl)[pl.MakeStruct(p.Name)], pl.EntryHook)
	dumpTables(b, u, pl.EntryHook, pl.RowHook)
	dumpProblems(b, pl.Problems())
}

// dumpProblems writes a plan's problems.
func dumpProblems(b *strings.Builder, problems []ir.GoNameProblem) {
	for _, pr := range problems {
		fmt.Fprintf(b, "  problem %d %s %s: %s, %s\n", pr.Kind, pr.Scope, pr.Name, pr.First, pr.Origin)
	}
}

// ownClasses are p's records, variants and dependent types, in declaration order.
func ownClasses(p *ir.Package) []any {
	var out []any
	for _, t := range p.Types {
		if _, enum := t.(*ir.Enum); !enum {
			out = append(out, t)
		}
	}
	return out
}

// classList names classes, a case under its variant.
func classList(u *ir.ForeignUse, classes []any) string {
	var out []string
	for _, c := range classes {
		if cs, ok := c.(*ir.Case); ok {
			out = append(out, u.VariantOf(cs).QName()+"."+cs.Name)
			continue
		}
		out = append(out, c.(ir.Type).QName())
	}
	return strings.Join(out, " ")
}

// goHooks prints the hooks of one class: a record's, each case's of a variant, each branch's of a dependent type; a case alone is its variant's.
func goHooks(pl *ir.GoNamePlan, u *ir.ForeignUse, c any) []string {
	var out []string
	switch x := c.(type) {
	case *ir.Record:
		out = append(out, goHook(pl.RecordHook(x)))
	case *ir.Variant:
		for _, cs := range x.Cases {
			out = append(out, goHook(pl.CaseHook(x, cs)))
		}
	case *ir.Dependent:
		for _, br := range x.Branches {
			out = append(out, pl.MakeBranchName(x, br))
		}
	}
	return out
}

// goHook prints a Go hook: its names, unwritten when its package writes none, and each slot with what it passes (K a key, ? a presence, D a define value, R a resolved getter the hook fills, cells a lookup's table).
func goHook(h ir.GoHook) string {
	var slots []string
	for _, s := range h.Slots {
		name := s.Slot.Getter
		var marks []string
		for _, m := range []struct {
			on   bool
			mark string
		}{{s.Slot.Resolved, "R"}, {s.Slot.Key, "K"}, {s.Slot.OK, "?"}, {s.Slot.Define, "D"}, {s.Finite != nil, "cells"}} {
			if m.on {
				marks = append(marks, m.mark)
			}
		}
		if len(marks) > 0 {
			name += "(" + strings.Join(marks, ",") + ")"
		}
		slots = append(slots, name)
	}
	names := h.Name
	if h.CaseType != "" {
		names += "/" + h.CaseType
	}
	if !h.Written {
		names += " unwritten"
	}
	return names + " [" + strings.Join(slots, " ") + "]"
}

// cppHooks is goHooks for C++.
func cppHooks(pl *ir.CppNamePlan, u *ir.ForeignUse, c any) []string {
	var out []string
	switch x := c.(type) {
	case *ir.Record:
		out = append(out, cppHook(pl.RecordHook(x)))
	case *ir.Variant:
		for _, cs := range x.Cases {
			out = append(out, cppHook(pl.CaseHook(x, cs)))
		}
	case *ir.Dependent:
		for _, br := range x.Branches {
			out = append(out, pl.MakeBranchName(x, br))
		}
	}
	return out
}

// cppHook prints a C++ hook: its names, unwritten when its package writes none, and each member, R where the hook fills the entry pointer, + its define member.
func cppHook(h ir.CppHook) string {
	var members []string
	for _, m := range h.Members {
		name := m.Member
		if m.Resolved {
			name += "(R)"
		}
		if m.DefineMember != "" {
			name += "+" + m.DefineMember
		}
		members = append(members, name)
	}
	names := h.Name
	if h.CaseType != "" {
		names += "/" + h.CaseType
	}
	if !h.Written {
		names += " unwritten"
	}
	return names + " [" + strings.Join(members, " ") + "]"
}

// goForwarded are the methods a Go row forwards: each slot's getters, a lookup's entry and key getters, each translated method.
func goForwarded(r ir.GoRow) string {
	var out []string
	for _, s := range r.Slots {
		slot, main := s.Slot, s.Slot.Main
		if s.Finite != nil {
			slot, main = s.Finite.Result, s.Finite.Result.Main
		}
		if main {
			out = append(out, slot.Getter)
		}
		if slot.Ref != nil {
			out = append(out, slot.KeyGetter)
		}
		if slot.Define {
			out = append(out, slot.ValueGetter)
		}
	}
	for _, fn := range r.Translated {
		out = append(out, fn.Name)
	}
	return strings.Join(out, " ")
}
