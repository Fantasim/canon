package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// reservedOnEnums are the member names an enum may not declare (E2105, TYPES.md §3.6).
var reservedOnEnums = []string{membersMember, typeNameMember, namedMember}

// namedParams are the parameters of `E.named(s: String)` (STDLIB.md §3, DECISIONS 372).
var namedParams = []*types.Param{{Name: paramS, Type: types.StringType}}

// enumQualifier is the enum a qualifier names, else nil (TYPES.md §4.3).
func (c *checker) enumQualifier(env *env, q *object) *types.EnumType {
	if q.kind == ObjPackage {
		return nil
	}
	e, ok := c.typeOfName(env, q, nil).(*types.EnumType)
	if ok {
		c.completeEnum(c.typeObjects[e], e) // TYPES.md §4.3: reflection sees every member
	}
	return e
}

// enumNamed is `E.named(s)`, typed `E?`; a wrong argument count, name or type is E3806 (TYPES.md §4.3, DECISIONS 372).
func (c *checker) enumNamed(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, e *types.EnumType) types.Type {
	c.info.NameUses[s.Name] = c.builtins[namedMember]
	result := &types.OptionalType{Elem: e}
	c.info.Types[s] = &types.FuncType{Params: []types.Type{types.StringType}, Result: result}
	fn := env.written(s)
	if len(x.Args) != 1 || x.Args[0].Name != nil && x.Args[0].Name.Name != paramS {
		c.wrongArity(env, x, fn, namedParams)
		c.typeArgsAlone(env, x, namedParams)
		return types.ErrorType
	}
	arg := x.Args[0].Value
	t := c.typedAgainst(env, arg, types.StringType)
	if _, fits := c.convert(t, types.StringType); !fits {
		c.wrongArity(env, arg, fn, namedParams)
		return types.ErrorType
	}
	c.accept(env, arg, t, types.StringType)
	c.info.Calls[x] = &Callee{Kind: CalleeBuiltin, Builtin: namedMember}
	return result
}

// typeFuncNamed is `F(a…).named(s)`: E3804, the enum is named instead (TYPES.md §4.3, DECISIONS 372); false for another call.
func (c *checker) typeFuncNamed(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr) bool {
	call, isCall := s.X.(*syntax.CallExpr)
	if !isCall || s.Optional || s.Name.Name != namedMember {
		return false
	}
	fn := c.typeFuncCallee(env, call)
	if fn == nil {
		return false
	}
	c.typeFuncReflection(env, s, call, fn)
	c.argsAlone(env, x)
	return true
}
