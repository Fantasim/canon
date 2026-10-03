package wire_test

import (
	"bytes"
	"context"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// catalogue names the types the findings cases decode against; `type` in a case picks one.
func catalogue(h *host) map[string]types.Type {
	config, task := heistia(h, newVocab(h))
	skill, _, _ := fpdemoSkill()
	bonus := record("game.items", "StatBonus", field("attribute", types.StringType), field("value", types.IntType))
	stats := field("stats", listOf(bonus))
	stats.Pairs, stats.WirePath = &types.Pairs{Keys: [2]string{"dwDestParam{i}", "nAdjParamVal{i}"}, Slots: 6}, nil
	level := record("features.csv", "LevelRow", field("level", types.IntType), field("exp", types.IntType, "exp_to_next"),
		h.withDefault(field("deathPenalty", types.IntType, "death_penalty"), num(0)))
	return map[string]types.Type{
		"pipeline.Potion":                        potionFixture(h),
		"resource.heistia.HeistiaConfig":         config,
		"[resource.heistia.Task]":                listOf(task),
		"[resource.heistia.Filter]":              listOf(filterFixture(task)),
		"resource.events.Event":                  eventFixture(h),
		"fpdemo.Skill":                           skill,
		"game.items.Item":                        record("game.items", "Item", field("id", types.StringType, "dwID"), stats),
		"teamboard.Intent":                       record("teamboard", "Intent", field("tone", tone)),
		"table flow.Status":                      &types.TableType{Elem: status},
		"{Int: Int}":                             &types.MapType{Key: types.IntType, Value: types.IntType},
		"[Int]":                                  listOf(types.IntType),
		"[features.csv.LevelRow] keyed by level": keyed(level, 0),
	}
}

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of decoding its files as `type`.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		h := newHost()
		named := catalogue(h)
		fs := &source.FileSet{}
		dec := wire.Decoder{Bag: diag.NewBag(fs, "p"), Pkg: "p", Host: h}
		var typ types.Type
		for _, f := range c.Archive.Files {
			if f.Name == "type" {
				typ = named[strings.TrimSpace(string(f.Data))]
			}
		}
		if typ == nil {
			t.Fatalf("%s: no known type", c.Path)
		}
		run := caseRun{dec: &dec, fs: fs, typ: typ}
		for _, f := range c.Archive.Files {
			run.decode(t, f.Name, f.Data)
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: dec.Bag.Summary(), Golden: true}
		if err := diag.Render(&buf, fs, dec.Bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected("findings.txt"))
}

// caseRun is one findings case: its decoder, files and type.
type caseRun struct {
	dec *wire.Decoder
	fs  *source.FileSet
	typ types.Type
}

// decode reads one archive file: JSON as a value, CSV with its header row.
func (c caseRun) decode(t *testing.T, name string, data []byte) {
	t.Helper()
	ext := path.Ext(name)
	if ext != ".json" && ext != ".csv" {
		return
	}
	f, err := c.fs.Add(name, "/"+name, data)
	if err != nil {
		t.Fatal(err)
	}
	var v value.Value
	var derr error
	if ext == ".csv" {
		rows := cells(f)
		v, _, derr = c.dec.CSV(context.Background(), rows[0], rows[1:], c.typ)
	} else if root, err := jsonsrc.Parse(f, c.dec.Bag); err == nil {
		v, _, derr = c.dec.Decode(context.Background(), wire.Selection{Node: root}, c.typ)
	}
	if derr != nil {
		t.Fatalf("%s: %v", name, derr)
	}
	reportForms(c.dec.Bag, v)
}

// reportForms reports, as verify does, each part of a decoded value Encode refuses (WIRE.md §5.1, E8102).
func reportForms(bag *diag.Bag, v value.Value) {
	switch x := v.(type) {
	case *value.Record:
		for i, f := range recordFields(x.T) {
			for _, form := range wire.FieldForms(f, x.Fields[i]) {
				form.At(spanOf(form.Site), f.Name).Report(bag)
			}
			reportForms(bag, x.Fields[i])
		}
	case *value.List:
		for _, e := range x.Elems {
			reportForms(bag, e)
		}
	case *value.Map:
		for _, e := range x.Vals {
			reportForms(bag, e)
		}
	case *value.Table:
		for _, e := range x.Entries {
			reportForms(bag, e)
		}
	}
}

// spanOf is where a decoded value was read, none without provenance.
func spanOf(v value.Value) source.Span {
	if p := v.Prov(); p != nil {
		return p.Span
	}
	return source.Span{}
}

// recordFields is the fields of a record or case type.
func recordFields(t types.Type) []*types.Field {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Fields
	case *types.CaseType:
		return d.Fields
	}
	return nil
}

// filterFixture is `record Filter { eventType: ref eventTypes  v: Param(eventType) | "all" }`.
func filterFixture(task *types.RecordType) *types.RecordType {
	dep := task.Fields[1].Type.(*types.OptionalType).Elem
	return record("resource.heistia", "Filter", field("eventType", task.Fields[0].Type),
		field("v", &types.LitUnionType{Of: dep, Literals: []string{"all"}}))
}
