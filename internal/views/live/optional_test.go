package live_test

import "testing"

// optional is a package whose type functions take optional arguments: an optional field, a path
// through a ref ending at an optional field, and a function without a `match`.
var optional = map[string]string{
	"a/a.canon": `package a

enum Goal { kill, collect }

record Mode {
  goal: Goal
}

let modes: table Mode = {
  hunt { goal: kill }
  gather { goal: collect }
}

type Target(m: Mode) = match m.goal {
  kill => String
  collect => Int
}

type Plain(m: Mode) = String

record Column {
  mode: ref modes?
}

let columns: table Column = {
  open { mode: none }
  hunting { mode: hunt }
}

record Role {
  column: ref columns
  mode: ref modes?
  aim: Target(mode)?
  via: Target(column.mode)?
  plain: Plain(mode)?
}

let roles: table Role = {
  free { column: open }
  hunter { column: hunting, mode: hunt, aim: "x", via: "y", plain: "z" }
  gatherer { column: hunting, mode: gather, aim: 1, via: "y", plain: "z" }
}
`,
}

// VIEWMODEL.md J14, TYPES.md 11.6, DECISIONS 307: a none driver selects no branch, a set one its own.
func TestTypesOptionalDriver(t *testing.T) {
	p := demo(t, "", optional)
	roles := p.let(t, "a", "roles")
	str, num := `{"kind":"string"}`, `{"kind":"int","bits":64,"signed":true}`
	got := []any{
		evaluate(t, p, entry(t, roles, "free"), "free", "").Types,
		evaluate(t, p, entry(t, roles, "hunter"), "hunter", "").Types,
		evaluate(t, p, entry(t, roles, "gatherer"), "gatherer", "").Types,
	}
	same(t, "VIEWMODEL.md J14 none driver", got, `[{},
	 {"aim":`+str+`,"via":`+str+`,"plain":`+str+`},
	 {"aim":`+num+`,"via":`+str+`,"plain":`+str+`}]`)
}
