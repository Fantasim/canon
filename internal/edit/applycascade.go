package edit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fieldCheck is a field a cascade judges once every operation is applied: an optional field
// whose type a changed field computes (E15).
type fieldCheck struct {
	t touched
	f *types.Field
}

// cascade applies E15 once every operation is applied, by re-typing (log-2026-09-29 M4 U4b-r):
// a field whose value does not fit the type its record's fields now compute, or that the final
// analysis refuses at its exact path, is dropped; nothing but the touched records (E16).
func (a *applier) cascade() error {
	checks := a.fieldChecks()
	if len(checks) == 0 {
		return nil
	}
	for _, c := range checks {
		if err := a.judge(c); err != nil {
			return err
		}
	}
	return nil
}

// fieldChecks are the fields E15 judges, in path order, each once.
func (a *applier) fieldChecks() []fieldCheck {
	var out []fieldCheck
	for _, t := range a.records {
		for _, f := range t.decl {
			if isOptional(f.Type) && dependsOnAny(f, t.decl, t.fields) {
				out = append(out, fieldCheck{t: t, f: f})
			}
		}
	}
	slices.SortStableFunc(out, func(x, y fieldCheck) int { return strings.Compare(x.path(), y.path()) })
	return slices.CompactFunc(out, func(x, y fieldCheck) bool { return x.path() == y.path() })
}

func (c fieldCheck) path() string {
	return childPath(c.t.path, Seg{Kind: SegField, Name: c.f.Name})
}

// dependsOnAny reports f's type computed from one of the named fields (TYPES.md §11).
func dependsOnAny(f *types.Field, fields []*types.Field, names []string) bool {
	for _, d := range f.DependsOn {
		if d < len(fields) && slices.Contains(names, fields[d].Name) {
			return true
		}
	}
	return false
}

// judge drops c's field when the final state refuses it: its value fits no longer the type
// its record's fields compute, or an error is reported at its exact place (a refinement, a
// `where`, an asset rule; a JSON source's decoding).
func (a *applier) judge(c fieldCheck) error {
	if a.restores(c.path()) {
		return nil
	}
	if err := a.settle(); err != nil {
		return err
	}
	res, err := a.resolve(c.path())
	if err == nil {
		if res.Target == nil || isNoneValue(res.Target) || a.fits(c, res) {
			return nil
		}
		return a.dropResolved(c, res)
	}
	if c.t.file == "" || !a.refusedAt(c) {
		return nil
	}
	return a.dropJSON(c)
}

// resolve reads a canonical path against the current state.
func (a *applier) resolve(path string) (resolution, error) {
	p, err := Parse(path)
	if err != nil {
		return resolution{}, err
	}
	return a.snap.open(p)
}

func isNoneValue(v value.Value) bool {
	_, none := v.(*value.None)
	return none
}

// fits reports c's value assignable to its type as its record computes it, refused nowhere (TYPES.md §6.2).
func (a *applier) fits(c fieldCheck, res resolution) bool {
	if a.refusedAt(c) {
		return false
	}
	rec, ok := res.parent(len(res.Steps) - 1).(*value.Record)
	if !ok {
		return true
	}
	fields := fieldsOf(rec.T) // this state's declaration: each analysis has its own types
	k := fieldIndex(fields, c.f.Name)
	if k < 0 {
		return true
	}
	ct, computed := concreteType(fields[k].Type, rec)
	return !computed || types.Assignable(res.Target.Type(), ct)
}

// refusedAt reports a finding of fitCodes the current analysis reports exactly at c's field: by
// path, or in a JSON source by its file and pointer.
func (a *applier) refusedAt(c fieldCheck) bool {
	pl, ok := a.placeOf(c)
	return ok && len(a.snap.refusals(pl, false)) > 0
}

// fitPlace is where an analysis reports that a field's value does not fit its type: its
// package and path relative to it, in a JSON source its file and member pointer, and in a
// .canon literal its item, where a static error has no path.
type fitPlace struct {
	pkg, rel, file, ptr string
	item                source.Span
}

// placeOf is c's field's place in the current state; false for a path that does not parse.
func (a *applier) placeOf(c fieldCheck) (fitPlace, bool) {
	p, err := Parse(c.path())
	if err != nil {
		return fitPlace{}, false
	}
	pl := fitPlace{pkg: p.Package, rel: Path{Root: p.Root, Segs: p.Segs}.String(), file: c.t.file}
	if n := a.memberNode(c); n != nil {
		pl.ptr = n.Pointer()
	}
	return pl, true
}

// refusals are the error findings of fitCodes s reports at pl: exactly at it, or with under at
// a value inside it too (a list's element, a map's value) and inside its item.
func (s *Snapshot) refusals(pl fitPlace, under bool) []diag.Finding {
	var out []diag.Finding
	for _, f := range s.a.Result().List {
		if !fitError(f) {
			continue
		}
		atPath := f.Package == pl.pkg && within(f.Path, pl.rel, pathMarks, under)
		atMember := pl.ptr != "" && within(f.Pointer, pl.ptr, pointerSep, under) && s.display(f.Span.File) == pl.file
		inItem := under && pl.item.End > pl.item.Start && f.Span.File == pl.item.File && pl.item.Cover(f.Span) == pl.item
		if atPath || atMember || inItem {
			out = append(out, f)
		}
	}
	return out
}

// within reports s at place, or with under inside it: past place, one of marks starts a step.
func within(s, place, marks string, under bool) bool {
	if s == place {
		return true
	}
	return under && place != "" && len(s) > len(place) && strings.HasPrefix(s, place) && strings.ContainsRune(marks, rune(s[len(place)]))
}

// memberNode is c's field in the JSON source that states its record, as edited so far; nil for
// a record no JSON source states, or a field its object lacks.
func (a *applier) memberNode(c fieldCheck) *jsonsrc.Node {
	if c.t.file == "" {
		return nil
	}
	root, err := a.jsonRoot(c.t.file)
	if err != nil {
		return nil
	}
	obj := root.Find(c.t.ptr)
	if obj == nil || obj.Kind != jsonsrc.Object {
		return nil
	}
	n, _, _ := memberAt(obj, c.f.WirePath)
	return n
}

// dropResolved drops a field the final state resolves: E15 sets it to none (E7 rules); its
// inverse sets it back as FromJSON after the driver (U4b-r).
func (a *applier) dropResolved(c fieldCheck, res resolution) error {
	raw, err := a.dropText(res.Target, c.f)
	if err != nil {
		return err
	}
	if err := a.run(Operation{Kind: OpSet, Path: c.path(), Value: None{}}, setOp); err != nil {
		if errors.Is(err, ErrNotEditable) {
			return nil // a value no edit can reach is left to the re-check
		}
		return err
	}
	a.reportDrop(c, raw)
	return nil
}

// reportDrop reports c's value as Dropped; its inverse follows the driver's (E22, E23).
func (a *applier) reportDrop(c fieldCheck, raw json.RawMessage) {
	a.dropped = append(a.dropped, Dropped{Path: c.path(), Value: raw})
	a.cascadeUndo = append(a.cascadeUndo, Operation{Kind: OpSet, Path: c.path(), Value: FromJSON(raw)})
}

// dropJSON drops a field of a JSON source whose value no longer decodes, which no path reaches:
// the member at its pointer is edited as E7 says (E15).
func (a *applier) dropJSON(c fieldCheck) error {
	n := a.memberNode(c)
	if n == nil {
		return nil
	}
	var b bytes.Buffer
	if err := json.Compact(&b, jsonsrc.Format(detached(n))); err != nil {
		return fmt.Errorf(fmtWrapped, errNoWire, err)
	}
	d := &jsonDiff{a: a}
	if c.f.NoneWire == nil && noneDefault(c.f) {
		d.remove(n, int(n.Span.Start))
	} else if err := d.set(&value.None{T: c.f.Type}, n, c.f); err != nil {
		return err
	}
	w := newWork()
	p, _ := Parse(c.path())
	w.addJSON(c.t.file, p.Package, d.out)
	if err := a.commit(w); err != nil {
		return err
	}
	a.reportDrop(c, b.Bytes())
	return nil
}

// noneDefault reports an optional field whose default is none: none written, or no default.
func noneDefault(f *types.Field) bool {
	if f.Default == nil {
		return true
	}
	_, none := syntax.Unparen(f.Default).(*syntax.NoneLit)
	return none
}
