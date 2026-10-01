package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkSafeInts is E8101: an integer a TypeScript emit writes fits a number, unless its field has @ts(bigint) (CODEGEN.md §4.1); not a value's own data in `types` mode, which emits none (decision 194; selectedValues), nor in a refused mode (decision 213).
func (s *stage) checkSafeInts(u *unit, es *emitSite) {
	if !modeRefused(es.e) {
		for _, v := range selectedValues(u, es.e) {
			span := v.span().span()
			s.unsafeInts(v.v.V, v.v.Name, false, func(n *value.Int, field string) {
				u.report(diag.E8101.At(span, n, field))
			})
		}
	}
	for _, c := range u.consts {
		s.unsafeInts(c.c.V, c.c.Name, false, func(n *value.Int, field string) {
			u.report(diag.E8101.At(c.span(), n, field))
		})
	}
}

// unsafeInts calls report for each integer of v outside ±(2^53 - 1); an element or map entry
// belongs to the field holding its collection.
func (s *stage) unsafeInts(v value.Value, field string, bigint bool, report func(*value.Int, string)) {
	switch x := v.(type) {
	case *value.Int:
		if !bigint && (x.V > maxSafeInt || x.V < -maxSafeInt) {
			report(x, field)
		}
	case *value.Record:
		s.unsafeIntsRecord(x, report)
	case *value.List:
		for _, e := range x.Elems {
			s.unsafeInts(e, field, bigint, report)
		}
	case *value.Map:
		for i := range x.Keys {
			s.unsafeInts(x.Keys[i], field, bigint, report)
			s.unsafeInts(x.Vals[i], field, bigint, report)
		}
	case *value.Table:
		for _, e := range x.Entries {
			s.unsafeInts(e, field, bigint, report)
		}
	}
}

// unsafeIntsRecord is unsafeInts's *value.Record case: each field checked under its own name
// and its own @ts(bigint) flag.
func (s *stage) unsafeIntsRecord(x *value.Record, report func(*value.Int, string)) {
	fields := s.fieldsOf(x.T)
	for i, fv := range x.Fields {
		if i < len(fields) {
			s.unsafeInts(fv, fields[i].Name, fields[i].BigInt, report)
		}
	}
}

// fieldsOf is the IR fields of the declaration a record value's type names.
func (s *stage) fieldsOf(t types.Type) []*Field {
	switch d := t.Base().(type) {
	case *types.RecordType:
		if r, ok := s.named[d].(*Record); ok {
			return r.Fields
		}
	case *types.AppliedRecord:
		return s.fieldsOf(d.Rec)
	case *types.CaseType:
		if v, ok := s.named[d.Variant].(*Variant); ok && d.Index < len(v.Cases) {
			return v.Cases[d.Index].Fields
		}
	}
	return nil
}

// checkDataLink is E8153: each value of a data-mode code emit, and each @reload value, is written by the package's emit json as `<value>.json` (WIRE.md §8.1).
func (s *stage) checkDataLink(u *unit) {
	written := writtenFiles(u)
	need := dataModeNames(u)
	for _, v := range u.values {
		file, ok := written[v.v.Name]
		switch {
		case !need[v.v.Name] && !v.v.Reload:
		case !ok:
			u.report(diag.E8153.AtNotWritten(v.span().span(), v.v.Name))
		case file != v.v.Name+JSONExt:
			u.report(diag.E8153.AtFileName(v.span().span(), v.v.Name))
		}
	}
}

// writtenFiles is the file each value name is written to by u's emit json, if any (WIRE.md §8.1).
func writtenFiles(u *unit) map[string]string {
	written := map[string]string{}
	for _, es := range u.emits {
		if es.e.Target != TargetJSON {
			continue
		}
		for _, name := range selectedNames(u, es.e) {
			if prev, ok := written[name]; ok && prev != name+JSONExt {
				continue // a copy already misnames it (CODEGEN.md §2.1: each copy is judged as an out)
			}
			written[name] = name + JSONExt
			if es.e.FileName != "" {
				written[name] = es.e.FileName
			}
		}
	}
	return written
}

// dataModeNames are the value names a data-mode code emit of u selects and emits: a value data mode refuses (E8015, CODEGEN.md §2.2) is not emitted in data mode, so its refusal is reported once.
func dataModeNames(u *unit) map[string]bool {
	need := map[string]bool{}
	for _, es := range u.emits {
		if !isCode(es.e.Target) || es.e.Mode != ModeData {
			continue
		}
		for _, v := range selectedValues(u, es.e) {
			if dataContainer(v.v.Type) {
				need[v.v.Name] = true
			}
		}
	}
	return need
}

// checkReload is E8202 (a go or cpp emit of an @reload value has a file to reload: data mode)
// and E8201 (an @reload value is not held by a legacy struct in fields or both access).
func (s *stage) checkReload(u *unit) {
	for _, v := range u.values {
		if !v.v.Reload {
			continue
		}
		for _, es := range u.emits {
			if es.index > 0 || es.e.Target != TargetGo && es.e.Target != TargetCpp || es.e.Mode == ModeData || modeRefused(es.e) || !selects(u, es.e, v.v.Name) {
				continue
			}
			u.report(diag.E8202.At(v.span().span(), v.v.Name, targetWords[es.e.Target], modeWords[es.e.Mode]))
		}
		if hasTarget(u, TargetCpp) {
			s.checkReloadStructs(u, v)
		}
	}
}

// checkReloadStructs is E8201: an `@reload` value's type, or any type it contains at any depth (a map, a variant case, a nested record of any package), maps onto a legacy C++ struct in `fields` or `both` access (SPEC.md §15.5).
func (s *stage) checkReloadStructs(u *unit, v *valueSite) {
	w := newWalker(func(n Type) bool {
		if rec, ok := n.(*Record); ok && (rec.Cpp.Access == AccessFields || rec.Cpp.Access == AccessBoth) {
			u.report(diag.E8201.At(v.span().span(), v.v.Name, rec.Name, rec.Cpp.Struct, accessWords[rec.Cpp.Access]))
		}
		return true
	}, func(*TypeRef) {})
	w.ref(&v.v.Type)
}

func selects(u *unit, e *Emit, name string) bool {
	return slices.Contains(selectedNames(u, e), name)
}

func hasTarget(u *unit, t Target) bool { return emitFor(u, t) != nil }
