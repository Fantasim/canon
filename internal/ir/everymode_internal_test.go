package ir

import (
	"reflect"
	"testing"
)

// everyModeProblem is a collision of the pair (a.Reward.by_ev, a.Reward.byEv) under name.
func everyModeProblem(name string) GoNameProblem {
	return GoNameProblem{Kind: GoCollision, Scope: "detail", Name: name, First: "a.Reward.by_ev", Origin: "a.Reward.byEv"}
}

// everyModeAlt is the collision of name with the pair's derived collision alt beside it (DECISIONS 458).
func everyModeAlt(name, alt string) GoNameProblem {
	pr, a := everyModeProblem(name), everyModeProblem(alt)
	a.Derived = true
	pr.Alt = &a
	return pr
}

// everyModeDerived is the derived collision of name, which stands in alone for a base collision.
func everyModeDerived(name string) GoNameProblem {
	pr := everyModeProblem(name)
	pr.Derived = true
	return pr
}

// DECISIONS 429, 458, 213: a refused mode reports what the plans of every valid mode have in common, the colliding pair of origins, once at plans[0]'s collision; the pair's derived Alt stays only when every mode has it, and stands in for a first collision that only some modes have.
func TestInEveryMode(t *testing.T) {
	const (
		getter = "GetByEv"
		table  = "kRewardByEvBranches"
	)
	tests := []struct {
		name  string
		plans [][]GoNameProblem
		want  []GoNameProblem
	}{
		{
			"an Alt only in plans[0] is dropped, the pair kept",
			[][]GoNameProblem{{everyModeAlt(getter, table)}, {everyModeProblem(getter)}, {everyModeProblem(getter)}},
			[]GoNameProblem{everyModeProblem(getter)},
		},
		{
			"an Alt only in a later plan is dropped",
			[][]GoNameProblem{{everyModeProblem(getter)}, {everyModeAlt(getter, table)}, {everyModeProblem(getter)}},
			[]GoNameProblem{everyModeProblem(getter)},
		},
		{
			"an Alt every mode has is kept",
			[][]GoNameProblem{{everyModeAlt(getter, table)}, {everyModeAlt(getter, table)}, {everyModeAlt(getter, table)}},
			[]GoNameProblem{everyModeAlt(getter, table)},
		},
		{
			"a first collision only plans[0] has: the Alt every mode has stands in",
			[][]GoNameProblem{{everyModeAlt(getter, table)}, {everyModeDerived(table)}, {everyModeDerived(table)}},
			[]GoNameProblem{everyModeDerived(table)},
		},
		{
			"a pair under a different name in each plan is reported once, at plans[0]'s",
			[][]GoNameProblem{{everyModeProblem("FooBar")}, {everyModeProblem("FooBarSchema")}, {everyModeProblem("FooBarSchema")}},
			[]GoNameProblem{everyModeProblem("FooBar")},
		},
		{
			"a pair under no common name loses an Alt the other modes lack",
			[][]GoNameProblem{{everyModeAlt("FooBar", table)}, {everyModeProblem("FooBarSchema")}, {everyModeProblem("FooBarSchema")}},
			[]GoNameProblem{everyModeProblem("FooBar")},
		},
		{
			"a pair a mode lacks is dropped",
			[][]GoNameProblem{{everyModeProblem(getter)}, {everyModeProblem(getter)}, nil},
			nil,
		},
		{
			"a name problem is no collision",
			[][]GoNameProblem{{{Kind: GoNotIdentifier, Name: "x"}}, {{Kind: GoNotIdentifier, Name: "x"}}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inEveryMode(tt.plans)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("inEveryMode = %+v, want %+v", got, tt.want)
			}
		})
	}
}
