package workspace

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// layered adds to o.Checked what each inactive layer of pkgs and their imports finds beyond the
// session: one cacheless analysis per layer, stacked last on the active ones, of the packages of
// pkgs importing its declarers, compared whole, cut once (API.md E18, F7, F11; DECISIONS 463).
func (o *EditOutcome) layered(ctx context.Context, checked *build.Analysis, pkgs []string, lang string) error {
	units, active := checked.Units(), checked.Layers()
	layers := inactiveLayers(units, importedBy(units, pkgs), active)
	if len(layers) == 0 {
		return nil
	}
	m := newMerge(checked)
	for _, layer := range layers {
		if err := ctx.Err(); err != nil {
			return err
		}
		b := o.After.b.WithLayers(append(slices.Clone(active), layer)).WithCache(nil)
		over := slices.DeleteFunc(importing(units, declaring(units, layer)...), func(p string) bool { return !slices.Contains(pkgs, p) })
		a, err := b.AnalyzeOnly(ctx, over)
		if err != nil {
			return err
		}
		for _, pkg := range a.Selected() {
			o.layerReads = append(o.layerReads, a.Consulted(pkg)...)
		}
		m.add(layer, a)
	}
	res, err := m.result(ctx, o.Checked, lang)
	o.Checked = res
	return err
}

// inactiveLayers are the layers the packages pkgs of units declare that active does not stack,
// in name order.
func inactiveLayers(units []*project.Unit, pkgs, active []string) []string {
	var out []string
	for _, u := range units {
		if slices.Contains(pkgs, u.Name) {
			out = append(out, u.Layers...)
		}
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return slices.DeleteFunc(out, func(l string) bool { return slices.Contains(active, l) })
}

// declaring are the packages of units that declare layer.
func declaring(units []*project.Unit, layer string) []string {
	var out []string
	for _, u := range units {
		if slices.Contains(u.Layers, layer) {
			out = append(out, u.Name)
		}
	}
	return out
}

// importedBy is pkgs with every package they import, directly or not, in units' order.
func importedBy(units []*project.Unit, pkgs []string) []string {
	in := map[string]bool{}
	for _, p := range pkgs {
		in[p] = true
	}
	for grown := true; grown; {
		grown = false
		for _, u := range units {
			grown = in[u.Name] && addAll(in, u.Imports) || grown
		}
	}
	var out []string
	for _, u := range units {
		if in[u.Name] {
			out = append(out, u.Name)
		}
	}
	return out
}

// addAll adds names to in, reporting one was not there.
func addAll(in map[string]bool, names []string) bool {
	grew := false
	for _, n := range names {
		grew = grew || !in[n]
		in[n] = true
	}
	return grew
}
