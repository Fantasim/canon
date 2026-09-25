package progen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// tyKind is a leaf type of the type-directed suite's model (IMPLEMENTATION-PLAN.md §7.7 item 3).
type tyKind uint8

const (
	tyInt tyKind = iota
	tyString
	tyBool
	tyFloat
	tyRange
	tyEnum
)

type wrapKind uint8

const (
	wrapNone wrapKind = iota
	wrapOptional
	wrapList
	wrapMap
)

// fieldType is one field's or root's type: a leaf kind, optionally wrapped, a range's bounds, and
// which enum ref names (an index into the model).
type fieldType struct {
	wrap   wrapKind
	kind   tyKind
	ref    int
	lo, hi int64
}

type enumDef struct {
	name    string
	members []string
}

// fieldSpec is one record field: its name, type and, sometimes, a default. Named apart from
// progen_test's recordField helper (ops_values_test.go), which builds a mutation site.
type fieldSpec struct {
	name string
	ft   fieldType
	def  *fieldDefault
}

// fieldDefault is a field default's Canon expression and the value it evaluates to (TYPES.md §15).
type fieldDefault struct {
	expr string
	val  any
}

type recordDef struct {
	name   string
	fields []fieldSpec
}

// typedModel is one generated program's declarations: its enums and flat records, CODEGEN.md §4.4.
type typedModel struct {
	enums   []enumDef
	records []recordDef
}

// leafKinds are the field kinds every record field and root may hold, wrapped or not.
var leafKinds = []tyKind{tyInt, tyString, tyBool, tyFloat, tyRange}

// scalarWraps are the wraps a scalar or enum field may take (never a record: keeps Go and JSON
// flat, sidestepping E8012's Range-in-a-wrapper and any need for nested smoke code).
var scalarWraps = []wrapKind{wrapNone, wrapNone, wrapOptional, wrapList, wrapMap}

// genModel builds n enums and m flat records; each record's fields are scalars, ranges, enum
// refs, or one of those wrapped optional, list or map.
func genModel(r *progen.Rand, n, m int) *typedModel {
	mo := &typedModel{}
	for i := range n {
		mo.enums = append(mo.enums, genEnum(r, i))
	}
	for i := range m {
		mo.records = append(mo.records, genRecord(r, mo, i))
	}
	return mo
}

func genEnum(r *progen.Rand, i int) enumDef {
	e := enumDef{name: fmt.Sprintf("E%d", i)}
	for k := 0; k < 2+r.Intn(3); k++ {
		e.members = append(e.members, fmt.Sprintf("m%d", k))
	}
	return e
}

func genRecord(r *progen.Rand, mo *typedModel, i int) recordDef {
	rec := recordDef{name: fmt.Sprintf("R%d", i)}
	for k := 0; k < 1+r.Intn(4); k++ {
		ft := pickFieldType(r, mo)
		f := fieldSpec{name: fmt.Sprintf("f%d", k), ft: ft}
		if canDefault(ft) && r.OneIn(2) {
			f.def = genDefault(r, mo, ft)
		}
		rec.fields = append(rec.fields, f)
	}
	return rec
}

// pickFieldType chooses a leaf kind (a range's bounds set when it is tyRange, an enum ref when
// the model has one) and whether it is wrapped.
func pickFieldType(r *progen.Rand, mo *typedModel) fieldType {
	kinds := leafKinds
	if len(mo.enums) > 0 {
		kinds = append(append([]tyKind(nil), leafKinds...), tyEnum)
	}
	kind := progen.Pick(r, kinds)
	ft := fieldType{kind: kind}
	if kind == tyEnum {
		ft.ref = r.Intn(len(mo.enums))
	}
	if kind == tyRange {
		ft.lo = int64(r.Intn(100))
		ft.hi = ft.lo + int64(1+r.Intn(900))
	}
	ft.wrap = progen.Pick(r, scalarWraps)
	return ft
}

// ensureDefault gives the first record a defaulted Int field when none of its fields has a
// default: every case then holds a value whose field the evaluator computes (genTypedRoots omits
// it from the first root), not only literals (DECISIONS 200 item 3).
func ensureDefault(r *progen.Rand, mo *typedModel) {
	rec := &mo.records[0]
	for _, f := range rec.fields {
		if f.def != nil {
			return
		}
	}
	ft := fieldType{kind: tyInt}
	rec.fields = append(rec.fields, fieldSpec{name: fmt.Sprintf("f%d", len(rec.fields)), ft: ft, def: genDefault(r, mo, ft)})
}
