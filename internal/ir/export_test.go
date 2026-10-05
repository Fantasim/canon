package ir

// GoScopeNames are the names the plan declared, by scope: the package, each struct, enum and container under its Go name and each table-read function's parameters.
func GoScopeNames(pl *GoNamePlan) map[string]map[string]bool { return scopeNames(pl.scopes) }

// CppScopeNames are the names a C++ plan declared, by scope: the namespace, detail, conformance, each class and enum.
func CppScopeNames(pl *CppNamePlan) map[string]map[string]bool { return scopeNames(pl.scopes) }

func scopeNames(scopes []*nameScope) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, sc := range scopes {
		if out[sc.what] == nil {
			out[sc.what] = map[string]bool{}
		}
		for name := range sc.names { //canon:unordered a set
			out[sc.what][name] = true
		}
	}
	return out
}

// TSModuleNames are the names the TypeScript plan of p's ts emit e declared in its module scope.
func TSModuleNames(p *Package, e *Emit) map[string]bool {
	return scopeNames(planTSNames(p, e).scopes)[tsModuleScope]
}

// EmitDefineRefs are the define tables emit e of p carries, as package.let, whether or not stage E could read them (CODEGEN.md §5.8).
func EmitDefineRefs(p *Package, e *Emit) []string {
	var out []string
	for _, d := range emitDefineRefs(p, e) {
		out = append(out, d.Pkg+qnameSep+d.Value)
	}
	return out
}
