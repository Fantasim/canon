package ir

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// genRule is one rule of what a generator cannot write, for one emit.
type genRule func(*stage, *unit, *emitSite)

// genRules are E8019 and E8020 per target and mode: only a mode a generator writes has any (decision 37; log-2026-09-24 "Revised").
var genRules [TargetView + 1][ModeTypes + 1][]genRule

func init() {
	common := []genRule{
		(*stage).checkFieldlessCaseFns, (*stage).checkOptionalElements,
		(*stage).checkOptionalMapValues, (*stage).checkTableFields, (*stage).checkCaseFields, (*stage).checkRecordConstants, (*stage).checkKindConstants,
		(*stage).checkNeverDependents, (*stage).checkDefineBranches, (*stage).checkVariantMembers,
	}
	goCode := append(slices.Clone(common), (*stage).checkForeignTables, (*stage).checkConstLiterals, (*stage).checkNegativeZero)
	genRules[TargetGo][ModeBaked] = append(slices.Clone(goCode), (*stage).checkBakedLiterals, (*stage).checkForeignTableLookups,
		(*stage).checkGoDependentLiterals, (*stage).checkDefineKeys)
	genRules[TargetGo][ModeData] = append(slices.Clone(goCode), (*stage).checkGoDecoded, (*stage).checkResolvedLookups,
		(*stage).checkGoDecodedDependents, (*stage).checkRefUnions)
	cppCode := append(slices.Clone(common), (*stage).checkCppDecoded, (*stage).checkCppDependents,
		(*stage).checkForeignPairs, (*stage).checkCppForeignRoots, (*stage).checkClassCycles, (*stage).checkSelfReads, (*stage).checkRefUnions)
	// ts takes the shared rules gen/ts needs (DECISIONS 278); it writes the other constructs: optional elements and map values, table fields, cases as types, record constants, a fieldless case's fns.
	tsCode := []genRule{(*stage).checkKindConstants, (*stage).checkNeverDependents, (*stage).checkDefineBranches, (*stage).checkVariantMembers, (*stage).checkTSForeignTables}
	tsBaked := append(slices.Clone(tsCode), (*stage).checkTSLiterals)
	genRules[TargetTS][ModeBaked], genRules[TargetTS][ModeEmbedded] = tsBaked, tsBaked
	tsDecode := append(slices.Clone(tsCode), (*stage).checkTSDecoded)
	genRules[TargetTS][ModeData], genRules[TargetTS][ModeTypes] = tsDecode, tsDecode
	genRules[TargetCpp][ModeData] = cppCode
	genRules[TargetCpp][ModeTypes] = append(slices.Clone(cppCode), (*stage).checkCppDefaults, (*stage).checkTypesInputs)
}

// checkGenSupport is E8019 and E8020: what the emit's generator refuses, so that check fails where build would (decision 37).
func (s *stage) checkGenSupport(u *unit, es *emitSite) {
	for _, rule := range genRules[es.e.Target][es.e.Mode] {
		rule(s, u, es)
	}
}

// reportGenConstruct is one E8019 finding, at the construct's own span, for the Kind an emit's generator cannot produce.
func (u *unit) reportGenConstruct(es *emitSite, span source.Span, kind diag.Kind) {
	u.report(diag.E8019.AtMode(span, targetWords[es.e.Target], modeWords[es.e.Mode], kind))
}

// checkFieldlessCaseFns is E8019 `FieldlessCaseExportFn`: a case without fields has no class for its export fns (CODEGEN.md §5.5).
func (s *stage) checkFieldlessCaseFns(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		v, ok := t.(*Variant)
		if !ok {
			continue
		}
		for _, c := range v.Cases {
			if len(c.Fields) == 0 && len(c.Methods) > 0 {
				u.reportGenConstruct(es, s.itemSpan(c, source.Span{}), diag.KindFieldlessCaseExportFn)
			}
		}
	}
}

// checkRecordConstants is E8019 `RecordConstant`: neither generator writes a constant of a record, variant or case type, nor gen/cpp one holding a record (CODEGEN.md §5.1).
func (s *stage) checkRecordConstants(u *unit, es *emitSite) {
	for _, c := range u.consts {
		if recordKinds[c.c.Type.Kind] || es.e.Target == TargetCpp && holdsRecordValue(c.c.V) {
			u.reportGenConstruct(es, c.span(), diag.KindRecordConstant)
		}
	}
}

// checkKindConstants is E8019 `VariantKindConstant` at each constant holding a variant's kind, itself or through its composites: no generator writes the kind enum's member as a constant yet (DECISIONS 292).
func (s *stage) checkKindConstants(u *unit, es *emitSite) {
	for _, c := range u.consts {
		if holdsVariantKind(c.obj.Type()) {
			u.reportGenConstruct(es, c.span(), diag.KindVariantKindConstant)
		}
	}
}

// holdsVariantKind reports a variant's kind type in t, itself or held by value (subTypes).
func holdsVariantKind(t types.Type) bool {
	b := t.Base()
	return b.Kind() == types.VariantKind || slices.ContainsFunc(subTypes(b), holdsVariantKind)
}

// holdsRecordValue reports a record or case value in v, itself or among its elements, keys and map values.
func holdsRecordValue(v value.Value) bool {
	switch x := v.(type) {
	case *value.Record:
		return true
	case *value.List:
		return slices.ContainsFunc(x.Elems, holdsRecordValue)
	case *value.Table:
		return len(x.Entries) > 0
	case *value.Map:
		return slices.ContainsFunc(x.Keys, holdsRecordValue) || slices.ContainsFunc(x.Vals, holdsRecordValue)
	default:
		return false
	}
}

// checkNegativeZero is E8020: a Go constant cannot hold -0.0, which gen/go writes as a `const` (CODEGEN.md §5.1).
func (s *stage) checkNegativeZero(u *unit, _ *emitSite) {
	for _, c := range u.consts {
		if f, ok := c.c.V.(*value.Float); ok && c.c.Type.Kind == types.Float && f.V == 0 && math.Signbit(f.V) {
			u.report(diag.E8020.At(c.span(), c.c.Name))
		}
	}
}
