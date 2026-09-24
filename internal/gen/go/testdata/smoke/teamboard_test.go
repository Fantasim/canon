package teamboard_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/sovereign15/sovcommon/roles"
	"gitlab.com/sovereign15/sovcommon/teamboard"
)

// Smoke test of the generated teamboard package: every value read through the getters.
func Example() {
	fmt.Println(teamboard.Version, teamboard.GetInitialStatus().Label(), teamboard.GetInitialStatusID())
	fmt.Println(teamboard.GetDefaultSeverityID(), teamboard.GetAssigneeMinRole(), teamboard.GetTriageMinRole())
	deck := teamboard.GetDeck()
	fmt.Println(deck.Layouts().Clone(), deck.MaxHidden())
	for s := range teamboard.GetStatuses().All() {
		by, ok := s.By()
		fmt.Println(s.ID(), s.Label(), s.Tone().Wire(), s.Terminal(), s.NextIDs().Clone(), s.Requires().Clone(), by, ok)
	}
	for c := range teamboard.GetColumns().All() {
		fmt.Println(c.ID(), c.Label(), c.Icon(), c.StatusesIDs().Clone(), c.AssigneeFilter(), c.Hideable())
	}
	sensitive, _ := teamboard.GetFlags().Find("sensitive")
	hint, _ := sensitive.Hint()
	view, _ := sensitive.ViewMinRole()
	fmt.Println(sensitive.ID(), hint, view, sensitive.SetBy())
	fmt.Println(teamboard.CanTransition(teamboard.StatusIDOpen, teamboard.StatusIDTaken),
		teamboard.CanTransition(teamboard.StatusIDVerified, teamboard.StatusIDFixed))
	fmt.Println(teamboard.ColumnOf(teamboard.StatusIDWontDo).Label())
	for sec := range teamboard.GroupedAreasVisibleTo(roles.RoleMaintainer).All() {
		fmt.Println(sec.GroupID(), sec.Group().Label(), sec.AreasIDs().Clone())
	}
	game := teamboard.GetAreas().Get(teamboard.AreaIDGame)
	fmt.Println(game.GroupID(), game.RoutesToIDs().Clone(), game.RoutesTo().At(0).Label())
	id, ok := teamboard.ParseStatusID("wont_do")
	fmt.Println(id, ok, teamboard.GetStatuses().Get(id) == teamboard.ColumnOf(id).Statuses().At(1))
	// Output:
	// 7 Open open
	// normal maintainer maintainer
	// [4:1 2:2] 2
	// open Open warning false [taken wont_do duplicate] [] reporter false
	// taken Taken info false [open fixed wont_do duplicate] [assignee] reporter false
	// fixed Fixed accent false [verified open] [] reporter false
	// verified Verified success true [open] [] reporter_or_triager true
	// wont_do Won't do neutral true [open] [reason] reporter false
	// duplicate Duplicate neutral true [open] [duplicate_of] reporter false
	// unclaimed Unclaimed inbox [open] false true
	// taken Taken hand [taken] true true
	// fixed Fixed, to verify check [fixed] false true
	// done Done archive [verified wont_do duplicate] false true
	// sensitive Exploits, dupes, security problems. maintainer reporter
	// true false
	// Done
	// in_game In game [game balance client resource]
	// out_game Out of game [website]
	// internal Internal [resource_studio team_board]
	// in_game [game_server client resource balance] Game server
	// wont_do true true
}

// A lookup built from the baked data is read through a pointer: no call copies its table.
func BenchmarkGroupedAreasVisibleTo(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = teamboard.GroupedAreasVisibleTo(roles.RoleMaintainer)
	}
}

func BenchmarkColumnOf(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = teamboard.ColumnOf(teamboard.StatusIDWontDo)
	}
}

// TestEveryGetter calls every getter of every entry, of every value and of what they return.
func TestEveryGetter(t *testing.T) {
	values := []any{
		teamboard.GetIntents(), teamboard.GetFlags(), teamboard.GetStatuses(), teamboard.GetColumns(),
		teamboard.GetSeverities(), teamboard.GetAreaGroups(), teamboard.GetAreas(), teamboard.GetDeck(),
		teamboard.GetInitialStatus(), teamboard.GetDefaultSeverity(),
	}
	calls := 0
	for _, v := range values {
		calls += walk(reflect.ValueOf(v), 3)
	}
	for r := range roles.RoleMembers() {
		calls += walk(reflect.ValueOf(teamboard.AreasVisibleTo(r)), 3)
		calls += walk(reflect.ValueOf(teamboard.GroupedAreasVisibleTo(r)), 3)
	}
	if calls < 500 {
		t.Errorf("%d getter calls", calls)
	}
}

// walk calls every exported method without arguments, and iterates All().
func walk(v reflect.Value, depth int) int {
	if depth == 0 || !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil()) {
		return 0
	}
	calls := 0
	for i := range v.NumMethod() {
		m := v.Type().Method(i)
		if m.Type.NumIn() != 1 || strings.HasPrefix(m.Name, "Clone") {
			continue
		}
		out := v.Method(i).Call(nil)
		calls++
		if m.Name == "All" {
			yield := reflect.MakeFunc(out[0].Type().In(0), func(args []reflect.Value) []reflect.Value {
				for _, a := range args {
					calls += walk(a, depth-1)
				}
				return []reflect.Value{reflect.ValueOf(true)}
			})
			out[0].Call([]reflect.Value{yield})
			continue
		}
		for _, o := range out {
			calls += walk(o, depth-1)
		}
	}
	return calls
}
