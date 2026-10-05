package world_test

import (
	"testing"

	core "example.com/data/game/core/out/go"
	corert "example.com/data/game/core/out/go/rt"
	p "example.com/data/game/world/out/go"
)

// CODEGEN.md §2.8, §5.14: world's baked literals call core's hooks: getters, stored results, the case-typed field, core's resolved ref.
func TestEmberfallBakedZone(t *testing.T) {
	z := p.GetZones().Get(p.ZoneIDForest)
	lv := z.Levels()
	if lv.Min() != 1 || lv.Max() != 5 || lv.Width() != 4 || lv.Scaled(true) != 8 || lv.Status().Label() != "Open" {
		t.Error("levels")
	}
	if n, ok := z.Named().Get("north"); !ok || n.Min() != 4 || z.Bands().At(0).StatusID() != core.StatusIDClosed {
		t.Error("named, bands")
	}
	if c, ok := z.Reward().AsCoins(); !ok || c.Amount() != 3 || z.Coins().Amount() != 7 || c.RankAt(true).N() != 2 || z.Coins().RankAt(false) != nil {
		t.Error("reward, coins")
	}
	if w, ok := z.Param().AsWord(); !ok || w != "wind" || z.Best().Max() != 12 {
		t.Error("param, best")
	}
	gate, ok := z.Markers().Find("gate")
	if !ok || gate.ID() != "gate" || gate.Code() != "g" || gate.Record().Code() != "g" || gate.HealFor(9) != 5 {
		t.Error("markers")
	}
	spots := z.Region().Spots() // core's own table: rows through core's entry hook
	if b, ok := spots.Find("b"); !ok || b.ID() != "b" || !b.Retired() || b.X() != 2 || z.Region().Name() != "r" {
		t.Error("region spots")
	}
}

// CODEGEN.md §5.9, §5.10: world's rows of core's record, and a lookup over core's table indexed by its id enum.
func TestEmberfallBakedRows(t *testing.T) {
	high := p.GetBands().Get(p.LevelRangeIDHigh)
	if high.ID() != p.LevelRangeIDHigh || !high.Retired() || high.Record().Min() != 6 || high.Status().Label() != "Closed" {
		t.Error("high")
	}
	if low, ok := p.GetBands().Find("low"); !ok || low.Retired() || low.Width() != 4 || low.Scaled(true) != 8 {
		t.Error("low")
	}
	if p.LevelOf(core.StatusIDClosed) != 20 || p.LevelOf(core.StatusIDOpen) != 10 {
		t.Error("levelOf")
	}
	if p.GetStart().Max() != 2 || p.Preset().At(0).Min() != 3 || p.GetStart().Rank().N() != 2 || p.Preset().At(0).Rank() != nil {
		t.Error("start, preset")
	}
	start := p.GetStart()
	if alts, ok := start.Alts(); !ok || alts.Len() != 2 || alts.At(0).N() != 1 {
		t.Error("start: a [ref ranks]? core resolves")
	}
	// log-2026-10-06 "U2 review FAIL" 1: an optional-result lookup's cells reach core's hook as values and presence.
	if _, ok := start.LabelFor(false); ok {
		t.Error("labelFor(false) is none")
	}
	if l, ok := start.LabelFor(true); !ok || l != "L" || start.RankFor(false) != nil || start.RankFor(true).N() != 1 {
		t.Error("labelFor, rankFor")
	}
	if start.StatusFor(true).Label() != "Closed" || start.StatusForID(false) != core.StatusIDOpen {
		t.Error("statusFor")
	}
	if start.BonusFor(core.StatusIDOpen) != 1 || start.BonusFor(core.StatusIDClosed) != 2 {
		t.Error("bonusFor: a lookup over core's table")
	}
	if rs, ok := start.RanksFor(core.StatusIDClosed); !ok || rs.Len() != 2 || rs.At(0).N() != 1 {
		t.Error("ranksFor")
	}
	if b := start.Boosts(); b.Len() != 1 || b.At(0).Rank().N() != 2 {
		t.Error("boosts")
	}
	if m, ok := start.Marks(); !ok || m.Len() != 1 {
		t.Error("marks")
	}
	// log-2026-10-06 "gen/go re-verify FAIL" 4: world's own lookup over core's table, held by no value.
	perk := p.Make_Perk("p", [2]int64{3, 4})
	if perk.CostFor(core.StatusIDClosed) != 4 {
		t.Error("perk")
	}
}

// CODEGEN.md §5.14: a hook builds a value without any check; core resolves a ref from its own data.
func TestEmberfallHooks(t *testing.T) {
	lv := core.Make_LevelRange(1, 3, corert.MakeList([]string{"x"}), core.ToneLoud, core.StatusIDClosed, "h", true, "r1", true, corert.MakeList([]string{"r2"}), true, corert.Map[string, core.StatusID]{}, false, corert.List[*core.Boost]{}, 2, [2]int64{2, 4},
		[2]string{"", "l"}, [2]bool{false, true}, [2]string{"", "r2"}, [2]bool{false, true}, [2]core.StatusID{core.StatusIDOpen, core.StatusIDClosed}, [2]int64{5, 6}, [2]corert.List[string]{}, [2]bool{})
	if lv.Status() != core.GetStatuses().Get(core.StatusIDClosed) || lv.StatusID() != core.StatusIDClosed {
		t.Error("core's hook resolves its ref into its own data")
	}
	if lv.Rank() != core.GetRanks().At(0) {
		t.Error("core's hook finds its keyed list entry")
	}
	if h, ok := lv.Hint(); !ok || h != "h" || lv.Scaled(true) != 4 || lv.Labels().At(0) != "x" {
		t.Error("parameters")
	}
	row := p.Make_LevelRangeRow(lv, p.LevelRangeIDLow, true)
	if row.ID() != p.LevelRangeIDLow || !row.Retired() || row.Min() != 1 {
		t.Error("row hook")
	}
	pw := core.Make_Param_Word("w")
	if w, ok := pw.AsWord(); !ok || w != "w" {
		t.Error("branch hook")
	}
	if r := core.Make_Reward_Nothing(); (&r).Kind() != core.RewardKindNothing {
		t.Error("case hook")
	}
}
