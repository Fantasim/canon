package shop_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	base "example.com/data/demo/base/out/go"
	shop "example.com/data/demo/shop/out/go"
)

func reload(t *testing.T) *shop.ShopSnapshot {
	t.Helper()
	if err := shop.Store.Reload("testdata/good"); err != nil {
		t.Fatal(err)
	}
	return shop.Store.Current()
}

// Fields of every form: sized numbers, int bools, units, codes, markers, paths, variants,
// a baked import's enum and table id, a key a struct tag cannot hold.
func TestItemFields(t *testing.T) {
	items := reload(t).Items()
	sword, ok := items.Find("sword")
	if !ok || sword.ID() != "sword" || sword.Retired() || sword.Code() != 300 || sword.Price() != 12.5 || sword.Weight() != 3.25 {
		t.Fatalf("sword: %+v", sword)
	}
	if !sword.Tradable() || sword.Tone() != shop.ToneSeries1 || sword.Element() != shop.ElementWater || sword.Delay() != 2*time.Second {
		t.Errorf("sword tradable, tone, element, delay: %+v", sword)
	}
	if _, has := sword.Note(); has {
		t.Error("the marker \"\" is none")
	}
	if _, has := sword.Bonus(); has {
		t.Error("null is none")
	}
	if o := sword.Origin(); o == nil || o.X() != -3 || o.Y() != 4 || sword.Path().Len() != 2 || sword.Path().At(1).Y() != 4 {
		t.Error("origin or path")
	}
	if fmt.Sprint(sword.Tags().Clone()) != "[sharp metal]" || sword.LegacyMax() != 9 || sword.Odd() != "weird" {
		t.Error("tags, legacy.max or odd,key")
	}
	if item, is := sword.Reward().AsItem(); !is || item.ItemID() != "II_GEM" || item.Count() != 5 || sword.Role() != base.RoleAdmin || sword.RankID() != base.RankIDHigh {
		t.Error("reward, role or rank")
	}
	shelf, has := sword.ShelfID()
	anvil := items.At(1)
	note, hasNote := anvil.Note()
	bonus, hasBonus := anvil.Bonus()
	if !has || shelf != "front" || !anvil.Retired() || note != "heavy" || !hasNote || bonus != -2 || !hasBonus || anvil.Origin() != nil {
		t.Error("shelf, retired, note, bonus")
	}
	if gold, is := anvil.Reward().AsGold(); !is || gold.Amount() != 9000000000 || items.At(2).Reward().Kind() != shop.RewardKindNothing {
		t.Error("gold or nothing")
	}
}

// Stored fns: getters, `$` tables in domain order, refs resolved inside the snapshot.
func TestStoredFns(t *testing.T) {
	items := reload(t).Items()
	sword, _ := items.Find("sword")
	anvil, _ := items.Find("anvil")
	nick, hasNick := sword.Nick()
	if sword.Heavy() || nick != "blade" || !hasNick || sword.Best() != anvil || sword.BestID() != "anvil" {
		t.Error("heavy, nick or best")
	}
	related, has := sword.Related()
	if !has || related.Len() != 2 || related.At(0).Label() != "Air" || related.At(1) != anvil {
		t.Error("related")
	}
	if _, has := anvil.Related(); has || sword.Score(shop.ToneInfo, true) != 11 || anvil.Score(shop.ToneLegacy, false) != 130 {
		t.Error("related none or score")
	}
	fire, hasFire := sword.LabelIn(shop.ElementFire)
	_, hasWater := sword.LabelIn(shop.ElementWater)
	front, hasFront := sword.ShelfForID(shop.ToneWarning)
	_, hasInfo := sword.ShelfForID(shop.ToneInfo)
	if fire != "fire" || !hasFire || hasWater || front != "front" || !hasFront || hasInfo {
		t.Error("labelIn or shelfFor")
	}
	air, _ := items.Find("air")
	if pair := sword.PairFor(shop.ToneWarning); pair != air || sword.PairFor(shop.ToneInfo) != nil || items.At(2).PairFor(shop.ToneInfo) != sword {
		t.Error("pairFor")
	}
	if kin := sword.Kin(true); kin.Len() != 2 || kin.At(1) != anvil || sword.Kin(false).Len() != 0 || anvil.Kin(false).At(0) != sword {
		t.Error("kin")
	}
	byCode, ok := items.FindByCode(12)
	if !ok || byCode.ID() != "air" {
		t.Error("FindByCode")
	}
}

// A @reload record resolves into the snapshot through an inline variant, a tree and a case.
func TestConfig(t *testing.T) {
	snap := reload(t)
	c, sword := snap.Config(), snap.Items().At(0)
	scale, hasScale := c.Scale()
	if c.Motd() != "Welcome" || c.Spawn().Y() != -20 || c.Flags().Len() != 2 || scale != 1.5 || !hasScale || c.Featured() != sword {
		t.Error("motd, spawn, flags, scale or featured")
	}
	if key, is := c.Event().AsItem(); !is || key.ItemID() != "II_KEY" || c.Picks().At(1) != sword || c.PicksIDs().At(0) != "air" {
		t.Error("event or picks")
	}
	alts, has := c.Alts()
	tree := c.Tree()
	if !has || alts.At(0).ID() != "anvil" || tree.Hot().ID() != "air" || tree.Next().Hot() != nil || tree.Kids().At(0).Hot() != sword {
		t.Error("alts or tree")
	}
	star, is := tree.Badge().AsStar()
	if !is || star.Of().ID() != "anvil" || tree.Kids().At(0).Badge().Kind() != shop.BadgeKindPlain {
		t.Error("badges")
	}
}

// A value that is not @reload loads alone; its refs into itself resolve, others stay keys.
func TestShelves(t *testing.T) {
	shelves, err := shop.LoadShelves("testdata/good/shelves.json")
	if err != nil {
		t.Fatal(err)
	}
	front, _ := shelves.Find("front")
	back, _ := shelves.Find("back")
	bonuses := front.Bonuses()
	if front.Slots() != -1 || front.Next() != back || back.Next() != nil || front.Delete() != 7 || front.I() != 30 || bonuses.Len() != 2 {
		t.Fatalf("front: %+v", front)
	}
	if bonuses.At(1).Stat() != shop.ToneInfo || bonuses.At(1).Amount() != 5 || fmt.Sprint(front.ItemsIDs().Clone()) != "[sword anvil]" {
		t.Error("bonuses or items")
	}
	if fmt.Sprint(front.Flags().Clone()) != "[a c]" || front.Waits().At(1) != 2*time.Second || back.Bonuses().Len() != 0 {
		t.Error("flags or waits")
	}
	link := front.Links().At(0)
	if !front.Lit() || back.Lit() || front.Depth() != 3 || front.Links().Len() != 1 || link.To() != back || link.ToID() != "back" || link.Note() != 7 {
		t.Error("lit, depth or links")
	}
	home, err := shop.LoadHome("testdata/good/home.json")
	if err != nil || home.X() != 5 || home.Y() != -6 {
		t.Errorf("home: %v", err)
	}
}

// Each failure names its file, row and key; a failed Reload keeps the snapshot.
func TestFailures(t *testing.T) {
	snap := reload(t)
	cases := []struct {
		err  error
		want []string
	}{
		{shop.Store.Reload("testdata/stale"), []string{"demo.shop.Item@ffffffff", shop.ItemsSchema}},
		{shop.Store.Reload("testdata/badsnap"), []string{"badsnap/items.json: rows[2].$best: no entry nope"}},
		{shop.Store.Reload("testdata/nowhere"), []string{"items.json"}},
		{load("testdata/bad/shelves.json"), []string{"testdata/bad/shelves.json: rows[0].next: no entry gone"}},
		{load("testdata/bad/tone.json"), []string{"testdata/bad/tone.json: rows[0].b0: unknown value loud"}},
		{load("testdata/bad/missing.json"), []string{"testdata/bad/missing.json: rows[0].slots: missing"}},
		{load("testdata/bad/int.json"), []string{"testdata/bad/int.json: rows[0].lit: expected 0 or 1, not 2"}},
		{load("testdata/bad/bits.json"), []string{"testdata/bad/bits.json: rows[0].flags: unknown bits 0x8"}},
		{load("testdata/bad/case.json"), []string{"testdata/bad/case.json: rows[0].Slots: differs from \"slots\" only in letter case"}},
		{load("testdata/bad/dup.json"), []string{"testdata/bad/dup.json: rows[0].i: duplicate key"}},
		{load("testdata/bad/pairs.json"), []string{"testdata/bad/pairs.json: rows[0].b1: pairs slot after an empty one"}},
		{load("testdata/bad/nullelem.json"), []string{"testdata/bad/nullelem.json: rows[0].waits[1]: null"}},
		{load("testdata/bad/nullpath.json"), []string{"testdata/bad/nullpath.json: rows[0].size: null"}},
		{load("testdata/bad/nullrow.json"), []string{"testdata/bad/nullrow.json: rows[0]: null"}},
		{load("testdata/bad/link.json"), []string{"testdata/bad/link.json: rows[0].l0: no entry gone"}},
		{load("testdata/bad/kind.json"), []string{"testdata/bad/kind.json: rows[0].slots: unexpected string"}},
		{load("testdata/bad/envelope.json"), []string{"testdata/bad/envelope.json: $Schema: differs from \"$schema\" only in letter case"}},
		{shop.Store.Reload("testdata/badcell"), []string{"badcell/items.json: rows[0].$kin.true[1]: no entry nope"}},
		{load("testdata/bad/pathcase.json"), []string{"testdata/bad/pathcase.json: rows[0].size.DEPTH: differs from \"depth\" only in letter case"}},
		{load("testdata/bad/pathcase2.json"), []string{"testdata/bad/pathcase2.json: rows[0].size.DEPTH: differs from \"depth\" only in letter case"}},
		{load("testdata/bad/deep.json"), []string{"testdata/bad/deep.json: rows[0].waits", "nested deeper than 512 levels"}},
		{load("testdata/bad/envrows.json"), []string{"testdata/bad/envrows.json: ROWS: differs from \"rows\" only in letter case"}},
		{shop.Store.Reload("testdata/nullcell"), []string{"nullcell/items.json: rows[0].$score.warning.false: null"}},
		{shop.Store.Reload("testdata/nulllistcell"), []string{"nulllistcell/items.json: rows[0].$kin.false: null"}},
		{shop.Store.Reload("testdata/nullrecelem"), []string{"nullrecelem/items.json: rows[0].path[1]: null"}},
		{shop.Store.Reload("testdata/nullid"), []string{"nullid/items.json: rows[0].$id: null"}},
		{shop.Store.Reload("testdata/domaincase"), []string{"domaincase/items.json: rows[0].$score.WARNING: differs from \"warning\" only in letter case"}},
	}
	for i, c := range cases {
		for _, w := range c.want {
			if c.err == nil || !strings.Contains(c.err.Error(), w) {
				t.Errorf("case %d: %v, want %q", i, c.err, w)
			}
		}
	}
	if shop.Store.Current() != snap {
		t.Error("a failed Reload replaced the snapshot")
	}
}

// Several keys break the case rule: the error names the smallest in byte order, every time (DOCTRINE §5).
func TestFirstKeyStable(t *testing.T) {
	want := "testdata/bad/cases.json: rows[0].DELETE: differs from \"delete\" only in letter case"
	for range 50 {
		if err := load("testdata/bad/cases.json"); err == nil || err.Error() != want {
			t.Fatalf("%v, want %s", err, want)
		}
	}
}

func load(path string) error {
	_, err := shop.LoadShelves(path)
	return err
}
