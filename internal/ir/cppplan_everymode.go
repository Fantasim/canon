package ir

// cppPlansOfEveryMode are the name plans of p's cpp emit e in every mode gen/cpp writes: what a
// mode check refused is judged on (DECISIONS 429).
func cppPlansOfEveryMode(p *Package, e *Emit) []*CppNamePlan {
	out := make([]*CppNamePlan, 0, len(cppModes))
	for _, m := range cppModes {
		out = append(out, PlanCppNames(p, inMode(e, m)))
	}
	return out
}

// refusedCppMode reports a cpp emit whose mode check refused: judged over every mode's plan, unlike an unbuilt one (DECISIONS 429).
func refusedCppMode(e *Emit) bool { return e.Mode == ModeNone }

// cppSharedKey3 is what tells one shared name from another across the plans of several modes.
type cppSharedKey3 struct{ scope, name, origin string }

// commonShared are the shared names of plans[0] that every plan declares, matched on scope, name and origin.
func commonShared(plans []*CppNamePlan) []cppShared {
	keyOf := func(n cppShared) cppSharedKey3 { return cppSharedKey3{n.scope, n.name, n.origin} }
	return inEvery(plans, func(pl *CppNamePlan) []cppShared { return pl.shared }, keyOf)
}

// commonDeclared are the names of plans[0] that every plan declares, matched on name, origin and way (declaredKey, CODEGEN.md §7.9).
func (s *stage) commonDeclared(plans []*CppNamePlan) []declaredName {
	return inEvery(plans, s.declaredNames, declaredName.key)
}

// inEvery are the items of plans[0] whose key every plan's items hold, in plans[0]'s order.
func inEvery[T any, K comparable](plans []*CppNamePlan, items func(*CppNamePlan) []T, key func(T) K) []T {
	if len(plans) == 0 {
		return nil
	}
	count := map[K]int{}
	for _, pl := range plans[1:] {
		seen := map[K]bool{}
		for _, it := range items(pl) {
			if k := key(it); !seen[k] {
				seen[k] = true
				count[k]++
			}
		}
	}
	var out []T
	for _, it := range items(plans[0]) {
		if count[key(it)] == len(plans)-1 {
			out = append(out, it)
		}
	}
	return out
}
