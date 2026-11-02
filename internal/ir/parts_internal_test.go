package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// CODEGEN.md §2.11, §3.5, DECISIONS 435: build<Last>_<N> is reserved in the package scope of a baked go emit. A name declared by hand pins the family and the collision's origins; the route a Canon program has, an import's Go package option, is findings/E8005_103 (an @go(name:) override must be exported, and lowerCamel writes no interior _).
func TestReserveBuildParts(t *testing.T) {
	const part = "buildA_0"
	e := &Emit{Target: TargetGo, Mode: ModeBaked, GoPackage: "a"}
	p := &Package{Name: "a", Emits: []*Emit{e}, Values: []*Value{{Name: "origin", Type: TypeRef{Kind: types.Int}}}}
	pl := PlanGoNames(p, e)
	if len(pl.Problems()) != 0 {
		t.Fatalf("problems before the declaration: %+v", pl.Problems())
	}
	pl.declare(pl.scopes[0], part, "a.rogue", nil)
	pl.reserveParts(pl.scopes[0])
	problems := pl.Problems()
	if len(problems) != 1 || problems[0].Kind != GoCollision || problems[0].Name != part || problems[0].First != "a.rogue" || problems[0].Origin != "a" {
		t.Errorf("problems = %+v, want one collision of %s between a.rogue and a", problems, part)
	}
}

// CODEGEN.md §2.11, §3.5, DECISIONS 435: a package with nothing baked reserves no build part.
func TestReserveBuildPartsNeedsBakedData(t *testing.T) {
	e := &Emit{Target: TargetGo, Mode: ModeBaked, GoPackage: "a"}
	p := &Package{Name: "a", Emits: []*Emit{e}}
	pl := PlanGoNames(p, e)
	pl.declare(pl.scopes[0], "buildA_0", "a.rogue", nil)
	pl.reserveParts(pl.scopes[0])
	if got := pl.Problems(); len(got) != 0 {
		t.Errorf("problems = %+v, want none", got)
	}
}
