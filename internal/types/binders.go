package types

import "slices"

// ChainArg is the argument parameter p is read from in declared type t, and the paths after (TYPES.md §11.4).
func ChainArg(t Type, p *Param) (*Arg, [][]*Field, bool) {
	chain := appChain(t, p, map[*TypeFunc]bool{})
	var paths [][]*Field
	for k, app := range slices.Backward(chain) {
		i := slices.Index(app.Fn.Params, p)
		if i < 0 || i >= len(app.Args) {
			return nil, nil, false
		}
		a := app.Args[i]
		if k == 0 {
			slices.Reverse(paths)
			return a, paths, true
		}
		if a.Source != ArgParam {
			return nil, nil, false
		}
		paths, p = append(paths, a.Path), a.Param
	}
	return nil, nil, false
}

// appChain is the applications from t down to one of p's function, outermost first, each
// function's body then arms searched once; nil when none reaches it.
func appChain(t Type, p *Param, seen map[*TypeFunc]bool) []*TypeAppType {
	if t == nil {
		return nil
	}
	switch x := t.Base().(type) {
	case *OptionalType:
		return appChain(x.Elem, p, seen)
	case *ListType:
		return appChain(x.Elem, p, seen)
	case *MapType:
		return appChain(x.Value, p, seen)
	case *DepMapType:
		return appChain(x.Value, p, seen)
	case *LitUnionType:
		return appChain(x.Of, p, seen)
	case *TypeAppType:
		return fnChain(x, p, seen)
	}
	return nil
}

// fnChain is app, then the applications its function's results reach p's function by.
func fnChain(app *TypeAppType, p *Param, seen map[*TypeFunc]bool) []*TypeAppType {
	fn := app.Fn
	if seen[fn] {
		return nil
	}
	seen[fn] = true
	if slices.Contains(fn.Params, p) {
		return []*TypeAppType{app}
	}
	results := []Type{fn.Body}
	for _, arm := range fn.Arms {
		results = append(results, arm.Result)
	}
	for _, res := range results {
		if rest := appChain(res, p, seen); rest != nil {
			return append([]*TypeAppType{app}, rest...)
		}
	}
	return nil
}

// MapBinder is the binder a map literal of type t binds to its own keys, "" for none (TYPES.md §11.5).
func MapBinder(t, declared Type, bound func(string) bool) string {
	switch x := t.Base().(type) {
	case *DepMapType:
		return x.Binder
	case *MapType:
		rt, isRef := x.Key.Base().(*RefType)
		if !isRef || declared == nil {
			return ""
		}
		if d := depMapOn(declared, rt.Target); d != nil && !bound(d.Binder) {
			return d.Binder
		}
	}
	return ""
}

// depMapOn is the first dependent map keyed by coll in t, through optionals, lists and map values.
func depMapOn(t Type, coll *Collection) *DepMapType {
	switch x := t.Base().(type) {
	case *OptionalType:
		return depMapOn(x.Elem, coll)
	case *ListType:
		return depMapOn(x.Elem, coll)
	case *MapType:
		return depMapOn(x.Value, coll)
	case *DepMapType:
		if x.Coll == coll {
			return x
		}
		return depMapOn(x.Value, coll)
	}
	return nil
}
