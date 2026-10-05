package progen_test

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// studioPackageName names the corpus's one studio package (examples/studio/studio.canon):
// widget declarations (G22) must live there, so E1629-E1631 mutate that file only.
const studioPackageName = "studio"

// VIEWMODEL.md G21, G22, N4.
func viewStudioOperators() []operator {
	return []operator{
		op(diag.E1629.Def().Code, "VIEWMODEL.md G22 (two default widgets for one type)", secondDefaultWidget),
		op(diag.E1630.Def().Code, "VIEWMODEL.md G22 (default widget on a disallowed type)", defaultWidgetBadType),
		op(diag.E1631.Def().Code, "VIEWMODEL.md G21 (siblings not a list of the value type)", siblingsMismatch),
		op(diag.W1640.Def().Code, "VIEWMODEL.md N4 (editable public value without a menu)", editableWithoutMenu),
	}
}

// isStudioSource tells tg the corpus's studio source file, where TimeOfDay is already imported
// and time_of_day is already a default widget for it (examples/studio/studio.canon).
func isStudioSource(tg target) bool { return isSource(tg) && tg.pkg == studioPackageName }

func secondDefaultWidget(tg target) []progen.Site {
	if !isStudioSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "/// Zz.\nwidget zz_e1629(value: TimeOfDay) default")}
}

func defaultWidgetBadType(tg target) []progen.Site {
	if !isStudioSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "/// Zz.\nwidget zz_e1630(value: Int) default")}
}

func siblingsMismatch(tg target) []progen.Site {
	if !isStudioSource(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "/// Zz.\nwidget zz_e1631(value: Int, siblings: String)")}
}

// editableWithoutMenu adds a public value without a menu, past the studio and data emits (N4).
func editableWithoutMenu(tg target) []progen.Site {
	if !isSource(tg) || !firstOfPackage(tg) || tg.pkg == studioPackageName || !packageHasEmitView(tg) {
		return nil
	}
	if hasDataEmit(tg) {
		return nil
	}
	return []progen.Site{appendBlock(tg, "/// Zz.\nrecord ZzW1640Item {\n  /// Zz.\n  label: String\n}\n\n"+
		"/// Zz.\nlet zzW1640: ZzW1640Item = { label: \"x\" }")}
}

// hasDataEmit tells tg's package emitting cpp or go in mode data or embedded (CODEGEN.md §2.1).
func hasDataEmit(tg target) bool {
	for _, p := range peers(tg) {
		for _, e := range nodes[*syntax.EmitDecl](p) {
			if emitModeIsData(p, e) {
				return true
			}
		}
	}
	return false
}

func emitModeIsData(p target, e *syntax.EmitDecl) bool {
	if e.Options == nil {
		return false
	}
	for _, it := range e.Options.Items {
		fi, ok := it.(*syntax.FieldItem)
		if ok && fi.Name.Name == "mode" {
			v := text(p, fi.Value)
			return v == "data" || v == "embedded"
		}
	}
	return false
}
