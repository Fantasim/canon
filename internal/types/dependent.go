package types

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// AppliedRecord is R(args), a parameterized record applied to arguments. It is a Record:
// arguments are erased statically (TYP-18) and bound at evaluation.
type AppliedRecord struct {
	Rec  *RecordType
	Args []*Arg
}

func (a *AppliedRecord) Kind() Kind { return Record }

func (a *AppliedRecord) Underlying() Type { return a }

func (a *AppliedRecord) Base() Type { return a }

// TypeAppType is F(args), a dependent type, its branch computed at verification (DEP-02).
type TypeAppType struct {
	Fn   *TypeFunc
	Args []*Arg
}

func (t *TypeAppType) Kind() Kind { return TypeApp }

func (t *TypeAppType) Underlying() Type { return t }

func (t *TypeAppType) Base() Type { return t }

// DepUnionType is the static view of a dependent value, printed F(*) (DEP-01).
type DepUnionType struct {
	Fn *TypeFunc
}

func (d *DepUnionType) Kind() Kind { return DepUnion }

func (d *DepUnionType) Underlying() Type { return d }

func (d *DepUnionType) Base() Type { return d }

// TypeFunc is a type alias with value parameters. Its body is either a type-level match
// (Scrutinee and Arms) or a plain type (Body).
type TypeFunc struct {
	Pkg, Name, Doc string
	Params         []*Param
	Scrutinee      *Scrutinee
	Arms           []*TypeArm
	Body           Type
	Decl           *syntax.TypeDecl
}

// Arm is the arm covering a member index (Bool: 0 false, 1 true), or nil.
func (f *TypeFunc) Arm(member int) *TypeArm {
	for _, a := range f.Arms {
		if a.Wildcard || slices.Contains(a.Members, member) {
			return a
		}
	}
	return nil
}

// Scrutinee is the path a type-level match reads, of enum or Bool Type: a parameter, fields.
type Scrutinee struct {
	Param *Param
	Path  []*Field
	Type  Type
}

// TypeArm is one arm of a type-level match: its members in pattern order, or `_`.
type TypeArm struct {
	Members  []int
	Wildcard bool
	Result   Type
}

// ArgSource says where a type argument is read from.
type ArgSource uint8

// Arg is an argument of a type application: a stable path rooted at a parameter of the
// enclosing declaration, an earlier field, or a dependent map's binder (DEP-05).
type Arg struct {
	Source ArgSource
	Param  *Param
	Binder string
	Path   []*Field // after the root; for ArgField, the earlier field first
}
