package ir

// GoScopeNames are the names the plan declared, by scope: the package, each struct, enum and container under its Go name and each table-read function's parameters.
func GoScopeNames(pl *GoNamePlan) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, sc := range pl.scopes {
		if out[sc.what] == nil {
			out[sc.what] = map[string]bool{}
		}
		for name := range sc.names { //canon:unordered a set
			out[sc.what][name] = true
		}
	}
	return out
}
