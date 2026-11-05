package build

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// LayerRun asks Layered for the declared layers an analysis does not activate (DECISIONS 463, 469).
type LayerRun struct {
	Pkgs []string // the packages the run checked
	Lang string   // the messages' language, "" for Options.Lang
	Own  bool     // compare Own's findings, an edit's re-check; else Result's, a check or a build
}

// layerPart is the analysis of one inactive layer.
type layerPart struct {
	layer string
	a     *Analysis
}

// Layered is res, a's result in lr.Lang, with what each inactive layer of lr.Pkgs and their imports
// finds beyond it, and the layers' analyses; res itself when there is no such layer or none adds
// (API.md E18, F7, F11; DECISIONS 463, 469).
func (a *Analysis) Layered(ctx context.Context, res *Result, lr LayerRun) (*Result, []*Analysis, error) {
	parts, err := a.layerParts(ctx, lr.Pkgs)
	analyses := make([]*Analysis, len(parts))
	for i, p := range parts {
		analyses[i] = p.a
	}
	if err != nil || len(parts) == 0 {
		return res, analyses, err
	}
	whole := (*Analysis).resultWhole
	if lr.Own {
		whole = (*Analysis).ownWhole
	}
	out, err := a.mergeLayers(whole, parts).result(ctx, res, lr.Lang)
	return out, analyses, err
}

// Merged is res, a's result in lang, with what layers, analyses Layered made of a's inactive
// layers, add beyond it, Result's findings compared: a check's merge without analysing again.
func (a *Analysis) Merged(ctx context.Context, res *Result, layers []*Analysis, lang string) (*Result, error) {
	if len(layers) == 0 {
		return res, nil
	}
	parts := make([]layerPart, 0, len(layers))
	for _, la := range layers {
		if stack := la.r.p.opt.Layers; len(stack) > 0 { // each layer is stacked last (layerParts); none is no layer's
			parts = append(parts, layerPart{layer: stack[len(stack)-1], a: la})
		}
	}
	return a.mergeLayers((*Analysis).resultWhole, parts).result(ctx, res, lang)
}

// layerParts analyses each inactive layer of pkgs and their imports, in name order, singly on the
// active ones, without the build cache (its lineages stay), over the packages of pkgs importing a
// declarer of the layer.
func (a *Analysis) layerParts(ctx context.Context, pkgs []string) ([]layerPart, error) {
	units, active := a.Units(), a.Layers()
	var out []layerPart
	for _, layer := range inactiveLayers(units, pkgs, active) {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		b := a.r.p.WithLayers(append(slices.Clone(active), layer)).WithCache(nil)
		la, err := b.AnalyzeOnly(ctx, unitNames(dependents(units, declaring(units, layer)), pkgs))
		if err != nil {
			return out, err
		}
		out = append(out, layerPart{layer: layer, a: la})
	}
	return out, nil
}

// mergeLayers is the merge of a's findings with the layers' parts, each read by whole.
func (a *Analysis) mergeLayers(whole func(*Analysis) []diag.Finding, parts []layerPart) *merge {
	m := newMerge(a, whole)
	for _, p := range parts {
		m.add(p.layer, p.a)
	}
	return m
}

// inactiveLayers are the layers pkgs of units and their imports declare that active does not
// stack, in name order.
func inactiveLayers(units []*project.Unit, pkgs, active []string) []string {
	var out []string
	for _, u := range imported(units, unitsNamed(units, pkgs)) {
		out = append(out, u.Layers...)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return slices.DeleteFunc(out, func(l string) bool { return slices.Contains(active, l) })
}

// declaring are the names of the packages of units that declare layer.
func declaring(units []*project.Unit, layer string) []string {
	var out []string
	for _, u := range units {
		if slices.Contains(u.Layers, layer) {
			out = append(out, u.Name)
		}
	}
	return out
}

// unitsNamed are the units of units that names holds, in units' order.
func unitsNamed(units []*project.Unit, names []string) []*project.Unit {
	return slices.DeleteFunc(slices.Clone(units), func(u *project.Unit) bool { return !slices.Contains(names, u.Name) })
}

// unitNames are the names of units that keep holds, in units' order.
func unitNames(units []*project.Unit, keep []string) []string {
	var out []string
	for _, u := range units {
		if slices.Contains(keep, u.Name) {
			out = append(out, u.Name)
		}
	}
	return out
}

// frozen is the done run r as an analysis whose bags are read whole, for a merge of its own
// findings with its layers' (r is private: no other call reads it).
func (r *run) frozen() *Analysis {
	return &Analysis{r: r, wholes: r.wholeBags()}
}

// layerErrors analyses the inactive layers of the build's selection, kept on r, and counts the
// errors they add to its findings, which block code, data and the lock (API.md B1a; DECISIONS 469).
func (r *run) layerErrors(ctx context.Context) (int, error) {
	base := r.frozen()
	parts, err := base.layerParts(ctx, r.selectedNames())
	if err != nil || len(parts) == 0 {
		return 0, err
	}
	r.inactive = parts
	return base.mergeLayers((*Analysis).resultWhole, parts).addedErrors(), nil
}

// checked is r's result in Options.Lang with what its inactive layers add (DECISIONS 469).
func (r *run) checked(ctx context.Context) (*Result, error) {
	res := r.result()
	if err := r.localized(ctx, res); err != nil {
		return nil, err
	}
	if len(r.inactive) == 0 {
		return res, nil
	}
	return r.frozen().mergeLayers((*Analysis).resultWhole, r.inactive).result(ctx, res, "")
}
