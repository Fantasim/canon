package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

func ExampleProject_Edit() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	// A new status of a stable table: its lock line is written with it (API.md E20).
	res, err := p.Edit(context.Background(), canon.Edit{
		Base: p.Revision(),
		Ops: []canon.Op{
			canon.Set("teamboard:statuses.open.label", canon.Str("Opened")),
			canon.AddEntry("teamboard:statuses", canon.Key("blocked"),
				canon.Source(`{ tone: danger, label: "Blocked", terminal: false, next: [open] }`)),
			canon.Set("teamboard:statuses.open.next", canon.List(canon.Key("taken"), canon.Key("blocked"))),
			canon.Set("teamboard:columns.taken.statuses", canon.List(canon.Key("taken"), canon.Key("blocked"))),
			canon.Set("teamboard:flags.blocking.hint", canon.Str("Someone waits on it")),
		},
		DryRun:   true,
		Evaluate: []string{"teamboard:statuses.open"},
	})
	if err != nil {
		explainEditError(err)
		return
	}
	var changes, undo []string
	for _, c := range res.Changes {
		changes = append(changes, describeChange(c))
	}
	for _, u := range res.Undo {
		undo = append(undo, undoLabel(u)+" "+u.Path)
	}
	fmt.Println(changes, len(res.Dropped), res.Applied, res.Revision == p.Revision(), res.Summary.Errors, len(res.Eval))
	fmt.Println(undo)
	// Output: [+teamboard/canon.lock ~teamboard/taxonomy.canon] 0 false true 0 1
	// [restore teamboard:flags.blocking.hint restore teamboard:columns.taken.statuses restore teamboard:statuses.open.next restore teamboard:statuses.open.label retire teamboard:statuses.blocked]
}

// ExampleSet builds operations with every kind of value (API.md §8.2).
func ExampleSet() {
	ops := []canon.Op{
		canon.Set("config.server.tls", canon.Bool(true)),
		canon.Set("balance.dropRate", canon.Float(0.25)),
		canon.Set("heistia.cooldown", canon.Dur(90*time.Second)),
		canon.Set("statuses.open.tone", canon.Member("danger")),
		canon.Set("quests.first.reward", canon.Case("item", canon.Obj{"item": canon.Key("II_GEN_GOLD")})),
		canon.Set("farm.modelTypes[3].unlocks", canon.List(canon.IntKey(4), canon.IntKey(5))),
		canon.Set("styles.weights", canon.Map(canon.KV{Key: canon.Member("kill"), Value: canon.Int(3)})),
		canon.Set("config.paths.iconDir", canon.None),
		canon.Reset("config.server.port"),
		canon.Insert("farm.modelTypes[3].levels", 0, canon.FromJSON([]byte(`{"level":1}`))),
		canon.Move("farm.modelTypes[3].levels[2]", 0),
		canon.Remove("farm.modelTypes[3].levels[1]"),
		canon.Retire("items.II_OLD_SWORD"),
		canon.SetCase("events[rain].kind", "spawn_item", nil),
	}
	kinds := make([]canon.OpKind, 0, len(ops))
	for _, op := range ops {
		kinds = append(kinds, op.Kind)
	}
	fmt.Println(kinds)
	// Output: [set set set set set set set set reset insert move remove retire setCase]
}

func ExampleSetCase() {
	op := canon.SetCase("eventConfig.events[moonstone_rain].kind", "spawn_item",
		canon.Obj{"spawnRegion": canon.Source("{ left: 0, top: 0, right: 10, bottom: 10 }")})
	fmt.Println(op.Kind, op.Path, op.Case)
	// Output: setCase eventConfig.events[moonstone_rain].kind spawn_item
}

func ExampleUnretire() {
	p, err := openExamples()
	if err != nil {
		return
	}
	defer p.Close()
	_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Unretire("teamboard:statuses.open")}})
	if errors.Is(err, canon.ErrStableKey) {
		fmt.Println("refused: a stable id is never un-retired")
	}
	// Output: refused: a stable id is never un-retired
}

// ExampleEdit reads an edit in the JSON form a web client sends (API.md §8.8).
func ExampleEdit() {
	data := []byte(`{"base":"r1:5f0c","ops":[{"op":"set","path":"farm.modelTypes[3].maxLevel","value":10}]}`)
	var e canon.Edit
	if err := json.Unmarshal(data, &e); err != nil {
		return
	}
	out, err := json.Marshal(e)
	if err != nil {
		return
	}
	fmt.Println(string(out))
	// Output: {"base":"r1:5f0c","ops":[{"op":"set","path":"farm.modelTypes[3].maxLevel","value":10}]}
}

// undoLabel names the button a studio shows for one op of EditResult.Undo (rule E23).
func undoLabel(op canon.Op) string {
	switch op.Kind {
	case canon.OpSet, canon.OpReset, canon.OpSetCase:
		return "restore"
	case canon.OpAdd, canon.OpInsert, canon.OpAddEntry:
		return "put back"
	case canon.OpRemove:
		return "remove"
	case canon.OpMove:
		return "move back"
	case canon.OpRename:
		return "rename back"
	case canon.OpRetire:
		return "retire" // an id added to a stable table stays, retired (rule E23)
	case canon.OpUnretire:
		return "never produced: retirement is one-way"
	}
	return string(op.Kind)
}

// describeChange names one file an edit wrote as a diff tool does: + created, - deleted, a
// renamed one by its two paths, ~ modified.
func describeChange(c canon.FileChange) string {
	switch c.Kind {
	case canon.Created:
		return "+" + c.Path
	case canon.Deleted:
		return "-" + c.Path
	case canon.Renamed:
		return c.OldPath + " -> " + c.Path
	}
	return "~" + c.Path
}

// explainEditError prints why an edit was refused, one case per error type of API.md §15.
func explainEditError(err error) {
	var (
		rejected  *canon.RejectedError
		stale     *canon.StaleError
		canonical *canon.NotCanonicalError
		readOnly  *canon.NotEditableError
		badValue  *canon.ValueError
		badPath   *canon.PathError
	)
	switch {
	case errors.As(err, &rejected):
		fmt.Println(len(rejected.Findings), "errors")
	case errors.As(err, &stale):
		fmt.Println("reload:", stale.Files)
	case errors.As(err, &canonical):
		fmt.Println("run canon fmt on", canonical.Files, "or set Normalize")
	case errors.As(err, &readOnly):
		fmt.Println("op", readOnly.Op, readOnly.Reason, readOnly.Origin, readOnly.Layer)
	case errors.As(err, &badValue):
		fmt.Println("op", badValue.Op, badValue.Path, badValue.Expected, badValue.Got)
	case errors.As(err, &badPath):
		explainPathError(badPath)
	}
}

// explainPathError tells the sentinels a *PathError of an edit may wrap apart.
func explainPathError(err *canon.PathError) {
	switch {
	case errors.Is(err, canon.ErrKeyExists), errors.Is(err, canon.ErrPathCollision):
		fmt.Println("choose another key:", err.Path)
	case errors.Is(err, canon.ErrStableKey):
		fmt.Println("retire it instead:", err.Path)
	case errors.Is(err, canon.ErrOverlay):
		fmt.Println("save the editor buffer first:", err.Path)
	case errors.Is(err, canon.ErrBadOp), errors.Is(err, canon.ErrNoPath):
		fmt.Println(err)
	}
}
