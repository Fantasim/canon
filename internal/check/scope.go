package check

import (
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// env is where an expression is checked: its declaration, scopes, body and facts (TYPES.md §6.6).
type env struct {
	pkg    *pkgState
	file   *syntax.File
	owner  *object
	scope  *scope
	rec    *recordCtx
	fn     *fnCtx
	facts  facts
	mode   mode
	it     *object
	params map[string]*object // type parameters of the enclosing declaration (TYPES.md §3.3)
	fields int                // a field default: the number of fields it may read (TYPES.md §15)
	what   diag.Kind          // a constant expression: what it is, for E3015
	base   *scope             // the innermost scope where a constant expression began

	shorthand types.Type // the receiver of a shorthand lambda's chain (TYPES.md §12.4)
	site      *object    // the field or let a value is stored in, for EVALUATION.md §4.3's related note

	magic     map[string]*object // a view's magic names in scope (VIEWMODEL.md §3.4)
	scopeFile *syntax.File       // the file whose imports are in scope, when not file (I18N.md T1)
	trans     *transCtx          // a translated template: its errors are E1703 (I18N.md T2)
}

// mode is a set of flags on an env.
type mode uint16

// scope is one block of local names (TYPES.md §3.4).
type scope struct {
	parent *scope
	names  map[string]*object
}

// recordCtx is a record or case body: `self`, its fields and its methods (TYPES.md §3.3 step 3).
type recordCtx struct {
	self    types.Type
	fields  map[string]*object
	methods map[string]*object
	order   []*object // fields in declaration order
}

// fnCtx is the function whose body is checked.
type fnCtx struct {
	name   string
	result types.Type
}

// fileEnv is the env of a declaration of p in file f, owned by o.
func (c *checker) fileEnv(p *pkgState, f *syntax.File, o *object) *env {
	return &env{pkg: p, file: f, owner: o, fields: -1, what: noConstant}
}

// with returns a copy of env, which the caller changes.
func (env *env) with() *env {
	e := *env
	return &e
}

// joining reports a branch synthesized for a join.
func (env *env) joining() bool { return env.mode&modeJoin != 0 }

// join is a copy of env synthesizing a branch for a join.
func (env *env) join() *env {
	e := env.with()
	e.mode |= modeJoin
	return e
}

// noJoin is a copy of env out of the join mode.
func (env *env) noJoin() *env {
	e := env.with()
	e.mode &^= modeJoin
	return e
}

// push is a copy of env with a new, empty block scope.
func (env *env) push() *env {
	e := env.with()
	e.scope = &scope{parent: env.scope, names: map[string]*object{}}
	return e
}

// declare binds a local name in the innermost scope: E2107 when that block already has it.
func (c *checker) declare(env *env, id *syntax.Ident, o *object) {
	c.info.Defs[id] = o
	if id.Name == syntax.Blank {
		return
	}
	if _, dup := env.scope.names[id.Name]; dup {
		c.report(env, diag.E2107.At(env.span(id), id.Name))
		return
	}
	env.scope.names[id.Name] = o
}

// newLocal makes the object of a local name of kind k (a param or any other binder).
func (c *checker) newLocal(env *env, k ObjKind, id *syntax.Ident, decl syntax.Node, t types.Type) *object {
	o := c.newObject(k, id.Name, env.pkg, decl, env.file)
	o.typ = t
	return o
}

// lookupLocal is step 2 of TYPES.md §3.3: the local scopes, innermost first.
func (env *env) lookupLocal(name string) *object {
	for s := env.scope; s != nil; s = s.parent {
		if o, ok := s.names[name]; ok {
			return o
		}
	}
	return nil
}

// lookupRecord is step 3: a field or a user method of the record or case body.
func (env *env) lookupRecord(name string) *object {
	if env.rec == nil {
		return nil
	}
	if o, ok := env.rec.fields[name]; ok {
		return o
	}
	return env.rec.methods[name]
}

// lookupGlobal is steps 4 to 6: the package, the file's imports, the built-ins.
func (c *checker) lookupGlobal(env *env, name string) *object {
	if o, ok := env.pkg.names[name]; ok {
		return o
	}
	if fs := env.pkg.scopes[env.scoped()]; fs != nil {
		if o, ok := fs.names[name]; ok {
			return o
		}
	}
	return c.universe[name]
}

// lookup is steps 2 to 6 of TYPES.md §3.3 for a name in value position.
func (c *checker) lookup(env *env, name string) *object {
	if o := env.lookupLocal(name); o != nil {
		return o
	}
	if env.rec != nil && env.rec.fields[name] != nil {
		return env.rec.fields[name]
	}
	if o := env.magic[name]; o != nil { // only a field or a parameter hides one (VIEWMODEL.md G11)
		return o
	}
	if o := env.lookupRecord(name); o != nil {
		return o
	}
	return c.lookupGlobal(env, name)
}

// scoped is the file whose imports env sees.
func (env *env) scoped() *syntax.File {
	if env.scopeFile != nil {
		return env.scopeFile
	}
	return env.file
}

// lookupType is the lookup in type position (TYPES.md §3.3): type parameters, package, imports, built-in types.
func (c *checker) lookupType(env *env, name string) *object {
	if o, ok := env.params[name]; ok {
		return o
	}
	return c.lookupGlobal(env, name)
}

// unknownName is E2102, with the closest spelling in scope as a hint.
func (c *checker) unknownName(env *env, n syntax.Node, name string) {
	if hint := c.closest(env, name); hint != "" {
		c.report(env, diag.E2102.AtHint(env.span(n), name, hint))
		return
	}
	c.report(env, diag.E2102.AtPlain(env.span(n), name))
}

// closest is the name in scope nearest to name by edit distance, within hintDistance and
// shorter than name; ties go to the smallest name in byte order.
func (c *checker) closest(env *env, name string) string {
	var names []string
	for s := env.scope; s != nil; s = s.parent {
		names = appendKeys(names, s.names)
	}
	if env.rec != nil {
		names = appendKeys(appendKeys(names, env.rec.fields), env.rec.methods)
	}
	names = c.globalNames(env, appendKeys(names, env.magic))
	slices.Sort(names)
	best, bestD := "", hintDistance+1
	for _, n := range names {
		if d := distance(name, n); d > 0 && d < bestD && d < len(name) {
			best, bestD = n, d
		}
	}
	return best
}

// globalNames appends the names of steps 4 to 6 to names.
func (c *checker) globalNames(env *env, names []string) []string {
	names = appendKeys(names, env.pkg.names)
	if fs := env.pkg.scopes[env.scoped()]; fs != nil {
		names = appendKeys(names, fs.names)
	}
	return appendKeys(names, c.universe)
}

func appendKeys(out []string, m map[string]*object) []string {
	return append(out, slices.Collect(maps.Keys(m))...)
}

// distance is the Levenshtein distance between a and b, in bytes.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := range len(a) {
		cur := make([]int, len(b)+1)
		cur[0] = i + 1
		for j := range len(b) {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}
			cur[j+1] = min(prev[j+1]+1, cur[j]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// localName drops the package from a qualified name of the finding's own package (ERRORS.md §1.3).
func (env *env) localName(text, pkg string) string {
	if pkg != env.pkg.path {
		return text
	}
	return strings.TrimPrefix(text, pkg+dot)
}

// storing is env for a value stored in the field or let o.
func (env *env) storing(o *object) *env {
	e := env.with()
	e.site = o
	return e
}
