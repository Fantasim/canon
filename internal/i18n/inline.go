package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// inlineLabel is a case field's label source found through a record's view (I18N.md K7): "the
// parent view only supplies its source text", the key stays the case's own (T.c.f).
type inlineLabel struct {
	vf   *syntax.ViewField
	file *syntax.File
}

// inlineLabels finds every case field's label found only via inlining (I18N.md K7).
func inlineLabels(pkgTypes []types.Type, byT map[types.Type]viewEntry, info *check.Info) map[*types.Field]inlineLabel {
	out := map[*types.Field]inlineLabel{}
	for _, t := range pkgTypes {
		rec, ok := t.(*types.RecordType)
		view := byT[t]
		if !ok || view.decl == nil {
			continue
		}
		addRecordInlineLabels(rec, view, info, out)
	}
	return out
}

// addRecordInlineLabels adds rec's view items that name a field of one of its inline variant
// fields' cases, skipping rec's own direct fields and methods, and a name check does not
// resolve (agreement with Info.NameUses, I18N.md K7).
func addRecordInlineLabels(rec *types.RecordType, view viewEntry, info *check.Info, out map[*types.Field]inlineLabel) {
	sc := walkItems(view.decl)
	own := ownNames(rec)
	for name, vf := range sc.fields { //canon:unordered each key set once, from its own name
		if own[name] || info.ObjectOf(vf.Name) == nil {
			continue
		}
		src := inlineLabel{vf: vf, file: view.file}
		addInlineField(rec.Fields, name, src, out, map[*types.VariantType]bool{})
	}
}

// ownNames is rec's own field and method names, the ones addRecordInlineLabels never overrides.
func ownNames(rec *types.RecordType) map[string]bool {
	out := make(map[string]bool, len(rec.Fields)+len(rec.Methods))
	for _, f := range rec.Fields {
		out[f.Name] = true
	}
	for _, m := range rec.Methods {
		out[m.Name] = true
	}
	return out
}

// addInlineField adds src for every case field named name, reached from fields' `@json(inline)`
// variant fields, depth first (as check's own inlineField/casesField, I18N.md K7).
func addInlineField(fields []*types.Field, name string, src inlineLabel, out map[*types.Field]inlineLabel, seen map[*types.VariantType]bool) {
	for _, f := range fields {
		vt, ok := f.Type.Base().(*types.VariantType)
		if !f.Inline || !ok || seen[vt] {
			continue
		}
		seen[vt] = true
		for _, ct := range vt.Cases {
			addCaseInlineField(ct, name, src, out, seen)
		}
	}
}

// addCaseInlineField adds src for ct's own field named name, and recurses into its own inline
// variant fields.
func addCaseInlineField(ct *types.CaseType, name string, src inlineLabel, out map[*types.Field]inlineLabel, seen map[*types.VariantType]bool) {
	for _, cf := range ct.Fields {
		if cf.Name == name {
			if _, exists := out[cf]; !exists {
				out[cf] = src
			}
		}
	}
	addInlineField(ct.Fields, name, src, out, seen)
}
