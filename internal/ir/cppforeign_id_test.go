package ir_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

const (
	foreignHintDefines = `package a

/// Words; the defines keep the names.
enum Word @cpp(defines: "") {
  /// One.
  Tiers
  /// Two.
  GetTiers
  /// Three.
  TierId
  /// Four.
  TierIdFromWire
  /// Five.
  tiers
  /// Six.
  best_ref
  /// Seven.
  LIMIT
  /// Eight.
  kMarksSchema
  /// Nine.
  LoadTop
  /// Ten.
  LoadCfg
  /// Eleven.
  live_
}

emit cpp { out: "@features/a", namespace: "a", mode: types }
`
	foreignHintBaked = `package b

/// A tier.
record Tier {
  /// Its rank.
  rank: Int
}

/// The tiers; each id is named like one of a's macros, and like another name of this table.
let tiers: table Tier = {
  Tiers { rank: 1 }
  GetTiers { rank: 2 }
  TierId { rank: 3 }
  TierIdFromWire { rank: 4 }
  tiers { rank: 5 }
  LIMIT { rank: 6 }
}

/// The best tier, a ref value: its Data member has a resolved twin.
let best: ref tiers = "LIMIT"

emit cpp { out: "@features/b", namespace: "b", mode: baked }
`
	foreignHintTiersJSON = `{ "Tiers": { "rank": 1 }, "GetTiers": { "rank": 2 }, "TierId": { "rank": 3 }, "TierIdFromWire": { "rank": 4 }, "tiers": { "rank": 5 }, "LIMIT": { "rank": 6 } }`
	foreignHintBestJSON  = `"LIMIT"`
	foreignHintData      = `package d

import b { Tier }

/// A mark.
record Mark {
  /// Its weight.
  weight: Int
}

/// The marks, loaded at run time: their schema constant is a's macro.
let marks: table Mark = {
  one { weight: 1 }
}

/// The top tier, a record of another package: its free loader is a's macro.
let top: Tier = { rank: 1 }

/// The settings: their access loader is a's macro.
let cfg: Mark = { weight: 2 }

/// The live mark: its snapshot member is a's macro.
@reload
let live: Mark = { weight: 3 }

emit cpp { out: "@features/d", namespace: "d", mode: data }
emit json { out: "out/" }
`
	foreignHintMarksJSON = `{ "one": { "weight": 1 } }`
	foreignHintTopJSON   = `{ "rank": 1 }`
	foreignHintCfgJSON   = `{ "weight": 2 }`
	foreignHintLiveJSON  = `{ "weight": 3 }`
	foreignHintMember    = `package c

/// Tones.
enum Tone {
  /// Spelled like a's define.
  LIMIT
}

emit cpp { out: "@features/c", namespace: "c", mode: types }
`
	foreignHintTable  = "b.tiers"
	foreignHintRecord = "b.Tier"
	foreignHintWord   = "a.Word."
)

// foreignHint is one expected E8005 finding: the variant, the name, the item it names.
type foreignHint struct {
	at   func(source.Span, string, string, string) *diag.Builder
	name string
	item string
}

// TestForeignDefineHints is CODEGEN.md §7.9 and ERRORS.md E8005 (DECISIONS 403; log-2026-11-01, E8005-ID and its review): one finding per (name, origin); a table id is `foreignId`, even named like another name of its table; a name derived from a value's own name that @cpp(name:) does not change (the container, k<V>Schema, Data::<v>, <v>_ref, the free Load<V>, detail::<P>Access::Load<V>, the snapshot member <v>_; log-2026-11-01 "E8005-ID fixes") is `foreignDerived`; <Rec>Id and <Rec>IdFromWire name the record; every other name (the accessor, an enum member) keeps `foreignDefine`. Each message is the registry's own variant.
func TestForeignDefineHints(t *testing.T) {
	w := newWorld(t)
	for _, f := range [][2]string{
		{"a/a.canon", foreignHintDefines}, {"b/b.canon", foreignHintBaked}, {"b.tiers.json", foreignHintTiersJSON},
		{"b.best.json", foreignHintBestJSON}, {"d/d.canon", foreignHintData}, {"d.marks.json", foreignHintMarksJSON},
		{"c/c.canon", foreignHintMember}, {"d.top.json", foreignHintTopJSON}, {"d.cfg.json", foreignHintCfgJSON},
		{"d.live.json", foreignHintLiveJSON},
	} {
		w.add(t, f[0], []byte(f[1]))
	}
	w.build(t)
	var got []string
	for _, pkg := range []string{"a", "b", "c", "d"} {
		for _, f := range w.bags[pkg].Findings() {
			got = append(got, string(f.Code)+" "+f.Message)
		}
	}
	id, derived, define := diag.E8005.AtForeignId, diag.E8005.AtForeignDerived, diag.E8005.AtForeignDefine
	want := expectedHints(w.fs, []foreignHint{
		{id, "Tiers", foreignHintTable + ".Tiers"}, {derived, "Tiers", foreignHintTable},
		{id, "GetTiers", foreignHintTable + ".GetTiers"}, {define, "GetTiers", foreignHintTable},
		{id, "TierId", foreignHintTable + ".TierId"}, {define, "TierId", foreignHintRecord},
		{id, "TierIdFromWire", foreignHintTable + ".TierIdFromWire"}, {define, "TierIdFromWire", foreignHintRecord},
		{id, "tiers", foreignHintTable + ".tiers"}, {derived, "tiers", foreignHintTable},
		{id, "LIMIT", foreignHintTable + ".LIMIT"}, {derived, "best_ref", "b.best"},
		{derived, "kMarksSchema", "d.marks"}, {define, "LIMIT", "c.Tone.LIMIT"},
		{derived, "LoadTop", "d.top"}, {derived, "LoadCfg", "d.cfg"}, {derived, "live_", "d.live"},
	})
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("findings\n%q\nwant\n%q", got, want)
	}
}

// expectedHints are the code and message each hint renders through the registry, sorted.
func expectedHints(files diag.Files, hints []foreignHint) []string {
	bag := diag.NewBag(files, "")
	for _, h := range hints {
		h.at(source.Span{}, h.name, h.item, foreignHintWord+h.name).Report(bag)
	}
	var out []string
	for _, f := range bag.Findings() {
		out = append(out, string(f.Code)+" "+f.Message)
	}
	slices.Sort(out)
	return out
}
