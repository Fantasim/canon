package ir

// GoInputs are the package-level names of runtime inputs (CODEGEN.md §5.12): LoadInputs, the flag every input getter checks, and LoadInputs' local of failures, escaped like a loader's (§3.4).
type GoInputs struct {
	Func, Loaded, Errs string
}

// GoInput is one input field's package slots: its value, its presence flag when it is optional, its compiled pattern when it has one ("" else).
type GoInput struct {
	Value, OK, Pattern string
}

// Inputs are LoadInputs' names; the plan declares them only when a record of the package has an input field.
func (pl *GoNamePlan) Inputs() GoInputs {
	errs, _ := pl.local(goInputErrs)
	return GoInputs{Func: loadInputs, Loaded: goInputsLoaded, Errs: errs}
}

// Input is field f of rec's slots, input_<T>_<store>: T is unique in the package and a store has no interior `_`, so two valid fields never share a slot (log-2026-09-24 "gen/go runtime inputs landed").
func (pl *GoNamePlan) Input(rec *Record, f *Field) GoInput {
	in := GoInput{Value: goInputPrefix + pl.TypeName(rec) + underscore + goEffectiveStore(f.Go, f.Name)}
	if f.Optional {
		in.OK = in.Value + goInputOKSuffix
	}
	if f.Pattern != nil {
		in.Pattern = in.Value + goInputPatternSuffix
	}
	return in
}

// inputRecords are p's records with an input field, in declaration order.
func inputRecords(p *Package) []*Record {
	var out []*Record
	for _, t := range p.Types {
		if r, ok := t.(*Record); ok && len(inputFields(r)) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// inputFields are rec's input fields, in declaration order.
func inputFields(rec *Record) []*Field {
	var out []*Field
	for _, f := range rec.Fields {
		if f.Input != nil {
			out = append(out, f)
		}
	}
	return out
}

// declareInputs declares every input's slots, the loaded flag and LoadInputs' local, as gen/go writes them after the package fns (CODEGEN.md §2.7, §5.12); declareAll declares LoadInputs.
func (pl *GoNamePlan) declareInputs(top *nameScope) {
	recs := inputRecords(pl.p)
	if len(recs) == 0 {
		return
	}
	for _, rec := range recs {
		for _, f := range inputFields(rec) {
			in := pl.Input(rec, f)
			pl.declareNonEmpty(top, rec.QName()+qnameSep+f.Name, f, in.Value, in.OK, in.Pattern)
		}
	}
	pl.declare(top, pl.Inputs().Loaded, pl.p.Name, nil)
	pl.declareFixedLocal(pl.scope(loadInputs+goParamsSuffix), goInputErrs)
}

// declareNonEmpty declares each of names that is not "" for item.
func (pl *GoNamePlan) declareNonEmpty(sc *nameScope, origin string, item any, names ...string) {
	for _, n := range names {
		if n != "" {
			pl.declare(sc, n, origin, item)
		}
	}
}

// inputImports marks what LoadInputs writes: rt and errors, and regexp for a pattern (CODEGEN.md §5.12, §6.3).
func (u *goImportUse) inputImports(fields []*Field) {
	for _, f := range fields {
		if f.Input != nil {
			u.mark(goRT, goErrors)
		}
		if f.Input != nil && f.Pattern != nil {
			u.mark(goRegexp)
		}
	}
}
