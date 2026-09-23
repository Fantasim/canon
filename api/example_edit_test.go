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
	res, err := p.Edit(context.Background(), canon.Edit{
		Base: p.Revision(),
		Ops: []canon.Op{
			canon.Set("farm.modelTypes[3].maxLevel", canon.Int(10)),
			canon.Add("farm.modelTypes[3].levels", canon.Obj{
				"level":             canon.Int(10),
				"modelName":         canon.Str("obj_UC017"),
				"productionItem":    canon.Key("II_GEN_MAT_MOONSTONE"),
				"productionPerHour": canon.Int(5),
			}),
			canon.AddEntry("statuses", canon.Key("blocked"),
				canon.Source(`{ tone: danger, label: "Blocked", terminal: false, next: [open] }`)),
			canon.Move("farm.modelTypes[3].levels[2]", 0),
			canon.Rename("statuses.blocked", canon.Key("on_hold")),
			canon.Reset("config.server.port"),
			canon.Set("config.paths.iconDir", canon.None),
		},
		DryRun:   true,
		Evaluate: []string{"teamboard:statuses.open"},
	})
	if err != nil {
		explainEditError(err)
		return
	}
	for _, c := range res.Changes {
		describeChange(c)
	}
	for _, d := range res.Dropped {
		fmt.Println("dropped", d.Path, string(d.Value))
	}
	for _, u := range res.Undo {
		fmt.Println(undoLabel(u), u.Path)
	}
	fmt.Println(res.Applied, res.Revision, res.Summary.Errors, len(res.Findings), len(res.Eval))
	// Output:
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
		canon.Insert("farm.modelTypes[3].levels", 0, canon.FromJSON([]byte(`{"level":1}`))),
		canon.Remove("farm.modelTypes[3].levels[1]"),
		canon.Retire("items.II_OLD_SWORD"),
		canon.SetCase("events[rain].kind", "spawn_item", nil),
	}
	kinds := make([]canon.OpKind, 0, len(ops))
	for _, op := range ops {
		kinds = append(kinds, op.Kind)
	}
	fmt.Println(kinds)
	// Output: [set set set set set set set insert remove retire setCase]
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
	_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Unretire("items.II_OLD_SWORD")}})
	if errors.Is(err, canon.ErrStableKey) {
		fmt.Println("refused: a stable id is never un-retired")
	}
	// Output:
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
	// Output:
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
	case canon.OpRetire, canon.OpUnretire:
		return "never produced: retirement is one-way"
	}
	return string(op.Kind)
}

// describeChange prints one file an edit wrote.
func describeChange(c canon.FileChange) {
	switch c.Kind {
	case canon.Modified, canon.Created:
		fmt.Println(c.Kind, c.Path, len(c.After))
	case canon.Deleted:
		fmt.Println(c.Kind, c.Path, len(c.Before))
	case canon.Renamed:
		fmt.Println(c.Kind, c.OldPath, "->", c.Path)
	}
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
