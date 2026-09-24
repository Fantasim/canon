// Package smoke is copied beside a temporary copy of examples/teamboard/expected/sovcommon by
// `make goldens-vet` (DECISIONS 201): it never lives in expected/, which holds only compiler
// output (M1 acceptance item 4, IMPLEMENTATION-PLAN.md M1).
package smoke

import (
	"reflect"
	"testing"

	"gitlab.com/sovereign15/sovcommon/teamboard"
)

// callGetters calls every exported, no-argument method of v: the generated code's own
// convention for a getter (CODEGEN.md), so a decode gap or a nil slice panics here.
func callGetters(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := range rt.NumMethod() {
		m := rt.Method(i)
		if m.Type.NumIn() != 1 || m.Type.NumOut() == 0 {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s.%s panicked: %v", rt, m.Name, r)
				}
			}()
			rv.Method(i).Call(nil)
		}()
	}
}

// TestSmoke reads every value of teamboard through its generated getters (M1 acceptance item 4):
// every table row, and the package-level singular values.
func TestSmoke(t *testing.T) {
	for row := range teamboard.GetIntents().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetFlags().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetStatuses().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetColumns().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetSeverities().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetAreaGroups().All() {
		callGetters(t, row)
	}
	for row := range teamboard.GetAreas().All() {
		callGetters(t, row)
	}
	callGetters(t, teamboard.GetDeck())
	callGetters(t, teamboard.GetInitialStatus())
	callGetters(t, teamboard.GetDefaultSeverity())
	if teamboard.GetStatuses().Len() == 0 || teamboard.GetIntents().Len() == 0 {
		t.Error("a table read as empty")
	}
}
