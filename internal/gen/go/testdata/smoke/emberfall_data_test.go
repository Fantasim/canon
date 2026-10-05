package world_test

import (
	"testing"

	core "example.com/data/game/core/out/go"
	p "example.com/data/game/world/out/go"
)

// CODEGEN.md §2.8, §5.14: world's readers read core's classes, which core's hooks build: getters, stored results, core's resolved ref.
func TestEmberfallZones(t *testing.T) {
	zones, err := p.LoadZones("testdata/good/zones.json")
	if err != nil {
		t.Fatal(err)
	}
	z, ok := zones.Find("forest")
	if !ok {
		t.Fatal("forest")
	}
	lv := z.Levels()
	if lv.Min() != 1 || lv.Max() != 5 || lv.Width() != 4 || lv.Scaled(true) != 8 || lv.Labels().At(0) != "a" {
		t.Errorf("levels: %d %d %d", lv.Min(), lv.Max(), lv.Width())
	}
	if lv.Status() == nil || lv.Status().Label() != "Open" || lv.StatusID() != core.StatusIDOpen {
		t.Error("levels: core's hook resolves its status")
	}
	if h, ok := z.Bands().At(0).Hint(); !ok || h != "low" || z.Bands().At(0).Status().Label() != "Closed" {
		t.Error("bands")
	}
	if n, ok := z.Named().Get("north"); !ok || n.Min() != 4 || n.Labels().At(0) != "n" {
		t.Error("named")
	}
	if c, ok := z.Reward().AsCoins(); !ok || c.Amount() != 3 || c.Rank() == nil || c.Rank().N() != 1 || c.RankAt(false) != nil || c.RankAt(true).N() != 2 || z.Reward().Kind() != core.RewardKindCoins {
		t.Error("reward")
	}
	if w, ok := z.Param().AsWord(); !ok || w != "wind" || z.Kind() != core.KindWord {
		t.Error("param")
	}
	if z.Best().Max() != 12 || z.Best().Scaled(false) != 3 {
		t.Error("best")
	}
}

// CODEGEN.md §5.9: a table field of core's records holds world's rows: ID, Retired, Record, forwarded getters.
func TestEmberfallMarkerRows(t *testing.T) {
	zones, err := p.LoadZones("testdata/good/zones.json")
	if err != nil {
		t.Fatal(err)
	}
	markers := zones.At(0).Markers()
	gate, ok := markers.Find("gate")
	if !ok || gate.ID() != "gate" || gate.Retired() || gate.Code() != "g" || gate.Record().Code() != "g" || gate.HealFor(3) != 3 {
		t.Error("gate")
	}
	door, ok := markers.Find("door")
	if !ok || !door.Retired() || markers.Len() != 2 {
		t.Error("door")
	}
	spots := zones.At(0).Region().Spots() // core's own table: rows through core's entry hook
	if b, ok := spots.Find("b"); !ok || b.ID() != "b" || !b.Retired() || b.X() != 2 || spots.At(0).ID() != "a" {
		t.Error("region spots")
	}
}

// CODEGEN.md §5.9: a table value of core's record has world's rows and id type; a record value of core's type is core's type.
func TestEmberfallBandsAndStart(t *testing.T) {
	bands, err := p.LoadBands("testdata/good/bands.json")
	if err != nil {
		t.Fatal(err)
	}
	high, ok := bands.Find("high")
	var id p.LevelRangeID = "high"
	if !ok || high.ID() != id || !high.Retired() || high.Min() != 6 || high.Record().Max() != 9 || high.Status().Label() != "Closed" {
		t.Error("high")
	}
	if high.Scaled(true) != 6 || high.Width() != 3 || high.StatusID() != core.StatusIDClosed {
		t.Error("forwarded methods")
	}
	start, err := p.LoadStart("testdata/good/start.json")
	if err != nil {
		t.Fatal(err)
	}
	if h, ok := start.Hint(); !ok || h != "go" || start.Labels().Len() != 2 {
		t.Error("start")
	}
	if r := start.Rank(); r == nil || r.N() != 2 {
		t.Error("start: core's hook finds its rank in its keyed list")
	}
	if b, ok := bands.Find("low"); !ok || b.Rank() != nil {
		t.Error("low: no rank")
	}
	if alts, ok := start.Alts(); !ok || alts.Len() != 2 || alts.At(1).N() != 2 {
		t.Error("start: a [ref ranks]? core resolves")
	}
	// log-2026-10-06 "U2 review FAIL" 1: an optional-result lookup's cells reach core's hook as values and presence.
	if _, ok := start.LabelFor(false); ok {
		t.Error("labelFor(false) is none")
	}
	if l, ok := start.LabelFor(true); !ok || l != "L" {
		t.Error("labelFor(true)")
	}
	if start.RankFor(false) != nil || start.RankFor(true).N() != 1 || start.StatusFor(true).Label() != "Closed" {
		t.Error("rankFor, statusFor: core resolves the cells")
	}
	if id, ok := start.RankForID(false); ok || id != "" {
		t.Error("rankForID(false) is none")
	}
	// log-2026-10-06 "U2 fixes done": a lookup over core's table reads its cells keyed by core's ids.
	if start.BonusFor(core.StatusIDOpen) != 1 || start.BonusFor(core.StatusIDClosed) != 2 {
		t.Error("bonusFor")
	}
	// log-2026-10-06 "gen/go re-verify FAIL": a [ref T]? lookup over core's table, a key-only map of refs, a pairs element core resolves.
	if _, ok := start.RanksFor(core.StatusIDOpen); ok {
		t.Error("ranksFor(open) is none")
	}
	if rs, ok := start.RanksFor(core.StatusIDClosed); !ok || rs.Len() != 2 || rs.At(1).N() != 2 {
		t.Error("ranksFor(closed)")
	}
	if m, ok := start.Marks(); !ok || m.Len() != 1 {
		t.Error("marks")
	}
	if b := start.Boosts(); b.Len() != 1 || b.At(0).Rank().N() != 2 || b.At(0).N() != 7 {
		t.Error("boosts")
	}
	if p.Preset().At(0).Min() != 3 || p.Preset().At(0).Status().Label() != "Closed" {
		t.Error("preset")
	}
}

// CODEGEN.md §2.8: world reads core's wire with its own strictness, a key core resolves to no entry is `no entry`.
func TestEmberfallBad(t *testing.T) {
	if _, err := p.LoadStart("testdata/bad/status.json"); err == nil || err.Error() != "testdata/bad/status.json: value.status: no entry nope" {
		t.Errorf("status: %v", err)
	}
	if _, err := p.LoadStart("testdata/bad/rank.json"); err == nil || err.Error() != "testdata/bad/rank.json: value.rank: no entry zz" {
		t.Errorf("rank: %v", err)
	}
	// log-2026-10-06 "U2 review FAIL" 2, 3: `no entry` covers a [ref T]? and lookup cells core resolves; a row of core's table field expects `$retired`.
	for file, want := range map[string]string{
		"alts":       "value.alts[1]: no entry zz",
		"rankcell":   "value.$rankFor.true: no entry zz",
		"statuscell": "value.$statusFor.false: no entry nope",
		"bonuscell":  "value.$bonusFor.closed: missing",
		"ranksfor":   "value.$ranksFor.closed[1]: no entry zz",
		"boost":      "value.b0: no entry zz",
		"mark":       "value.marks.a: no entry nope",
	} {
		path := "testdata/bad/" + file + ".json"
		if _, err := p.LoadStart(path); err == nil || err.Error() != path+": "+want {
			t.Errorf("%s: %v", file, err)
		}
	}
	if _, err := p.LoadZones("testdata/bad/retired.json"); err == nil || err.Error() != `testdata/bad/retired.json: rows[0].region.spots.b.$Retired: differs from "$retired" only in letter case` {
		t.Errorf("retired: %v", err)
	}
	if _, err := p.LoadZones("testdata/bad/rankat.json"); err == nil || err.Error() != "testdata/bad/rankat.json: rows[0].reward.$rankAt.true: no entry zz" {
		t.Errorf("rankat: %v", err)
	}
	if _, err := p.LoadZones("testdata/bad/coinsrank.json"); err == nil || err.Error() != "testdata/bad/coinsrank.json: rows[0].reward.rank: no entry zz" {
		t.Errorf("coinsrank: %v", err)
	}
	if _, err := p.LoadZones("testdata/bad/reward.json"); err == nil || err.Error() != "testdata/bad/reward.json: rows[0].reward.type: unknown case gems" {
		t.Errorf("reward: %v", err)
	}
	if _, err := p.LoadBands("testdata/bad/case.json"); err == nil || err.Error() != `testdata/bad/case.json: rows[0].Status: differs from "status" only in letter case` {
		t.Errorf("case: %v", err)
	}
}
