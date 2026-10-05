package genmapdata_test

import (
	"slices"
	"testing"

	p "example.com/data/genmapdata/out/go"
)

// WIRE.md §5.8, CODEGEN.md §5.9, DECISIONS 312: a map is read in file order, each key by its wire form.
func TestMapFieldGood(t *testing.T) {
	boards, err := p.LoadBoards("testdata/good/board.json")
	if err != nil {
		t.Fatal(err)
	}
	b := boards.At(0)
	var power []p.Element
	for k := range b.Power().All() {
		power = append(power, k)
	}
	if !slices.Equal(power, []p.Element{p.ElementWater, p.ElementFire}) {
		t.Errorf("power keys %v, want file order water, fire", power)
	}
	if n, ok := b.Power().Get(p.ElementFire); !ok || n != 5 {
		t.Errorf("fire = %d, %v", n, ok)
	}
	var names []string
	for k := range b.Names().All() {
		names = append(names, k)
	}
	if !slices.Equal(names, []string{"z", "a"}) {
		t.Errorf("names keys %v", names)
	}
	var levels []int64
	for k := range b.Levels().All() {
		levels = append(levels, k)
	}
	if !slices.Equal(levels, []int64{10, -3, 2}) {
		t.Errorf("levels keys %v, want file order 10, -3, 2", levels)
	}
	if s, ok := b.Levels().Get(-3); !ok || s != "minus" {
		t.Errorf("levels[-3] = %q, %v", s, ok)
	}
	if g, ok := b.Groups().Get("b"); !ok || g.Len() != 2 {
		t.Errorf("groups[b] = %v, %v", g, ok)
	}
	if extra, ok := b.Extra(); !ok || extra.Len() != 1 {
		t.Errorf("extra = %v, %v", extra, ok)
	}
}

// CODEGEN.md §5.8: a ref in a record a map holds is resolved at load, to the board it names.
func TestMapFieldRefsResolve(t *testing.T) {
	boards, err := p.LoadBoards("testdata/good/board.json")
	if err != nil {
		t.Fatal(err)
	}
	slots := boards.At(0).Slots()
	y, ok := slots.Get("y")
	if !ok || y.Home() != boards.At(1) {
		t.Errorf("slot y home = %v, %v, want the second board", y.Home(), ok)
	}
	if x, _ := slots.Get("x"); x.Home() != boards.At(0) {
		t.Errorf("slot x home = %v, want the first board", x.Home())
	}
}

// An empty object is an empty map; an absent optional map is none.
func TestMapFieldEmpty(t *testing.T) {
	boards, err := p.LoadBoards("testdata/good/empty.json")
	if err != nil {
		t.Fatal(err)
	}
	b := boards.At(0)
	if b.Power().Len() != 0 || b.Levels().Len() != 0 {
		t.Error("maps are not empty")
	}
	if _, ok := b.Extra(); ok {
		t.Error("extra is present")
	}
}

// WIRE.md §5.8 (E7103, E7111, E3501): a key that is not a canonical integer (`01`, `+1`, `-0`, empty), one out of its type's range, an unknown enum member or code, a ref key to no board, a value of the wrong kind, a non-object and a repeated key fail the load at their place, the path showing each key as the file wrote it; a resolve failure under an enum or a @json(codes) key too.
func TestMapFieldBad(t *testing.T) {
	for file, want := range map[string]string{
		"intkey":     `rows[0].levels.01: expected a canonical decimal integer key`,
		"plus":       `rows[0].levels.+1: expected a canonical decimal integer key`,
		"negzero":    `rows[0].small.-0: expected a canonical decimal integer key`,
		"emptykey":   `rows[0].small: expected a canonical decimal integer key`,
		"emptyname":  `rows[0].names: expected a string`,
		"range":      `rows[0].small.300: expected an integer from -128 to 127`,
		"huge":       `rows[0].levels.99999999999999999999: expected an integer`,
		"enumkey":    `rows[0].power.earth: unknown value earth`,
		"rankcode":   `rows[0].ranks.3: unknown value 3`,
		"rankname":   `rows[0].ranks.low: expected a canonical decimal integer key`,
		"tonekey":    `rows[0].tones.loud: unknown value loud`,
		"moodvalue":  `rows[0].moods.a: unknown value loud`,
		"resultkey":  `rows[0].$byBoardTotals.zz: no entry zz`,
		"litnohome":  `rows[0].byLit.all.home: no entry zz`,
		"pertone":    `rows[0].$perTone.series-1.zz: no entry zz`,
		"boardkey":   `rows[0].byBoard.zz: no entry zz`,
		"value":      `rows[0].power.fire: expected an integer`,
		"deepkey":    `rows[0].deep.q.x: expected a canonical decimal integer key`,
		"deepvalue":  `rows[0].deep.q.1: expected a string`,
		"totalskey":  `rows[0].$totals.a: expected an integer`,
		"array":      `rows[0].power: expected an object`,
		"dup":        `rows[0].power.fire: duplicate key`,
		"nohome":     `rows[0].slots.s.home: no entry nowhere`,
		"ranknohome": `rows[0].rankSlots.200.home: no entry nowhere`,
		"elemnohome": `rows[0].elemSlots.water.home: no entry nowhere`,
	} {
		_, err := p.LoadBoards("testdata/bad/" + file + ".json")
		if want = "testdata/bad/" + file + ".json: " + want; err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %s", file, err, want)
		}
	}
}

// WIRE.md §5.8: the keys of every kind read in file order, an enum with @json(codes) by its code, a literal union by its string, a ref by its key, nested maps and a stored fn's map too.
func TestMapFieldKinds(t *testing.T) {
	boards, err := p.LoadBoards("testdata/good/board.json")
	if err != nil {
		t.Fatal(err)
	}
	b := boards.At(0)
	var ranks []p.Rank
	for k := range b.Ranks().All() {
		ranks = append(ranks, k)
	}
	if !slices.Equal(ranks, []p.Rank{p.RankHigh, p.RankLow}) {
		t.Errorf("ranks keys %v, want file order high, low", ranks)
	}
	var tones []string
	for k := range b.Tones().All() {
		tones = append(tones, k)
	}
	if !slices.Equal(tones, []string{"series-1", "all", "plain"}) {
		t.Errorf("tones keys %v", tones)
	}
	if m, ok := b.Moods().Get("b"); !ok || m != "series-1" {
		t.Errorf("moods[b] = %q, %v", m, ok)
	}
	if n, ok := b.Small().Get(-5); !ok || n != "m" {
		t.Errorf("small[-5] = %q, %v", n, ok)
	}
	if n, ok := b.ByBoard().Get("b1"); !ok || n != 2 {
		t.Errorf("byBoard[b1] = %d, %v", n, ok)
	}
	if n, ok := b.ByNode().Get(-2); !ok || n != "x" {
		t.Errorf("byNode[-2] = %q, %v", n, ok)
	}
	if inner, ok := b.Deep().Get("q"); !ok || inner.Len() != 2 {
		t.Errorf("deep[q] = %v, %v", inner, ok)
	}
	if n, ok := b.Totals().Get("a"); !ok || n != 2 || b.Totals().Len() != 2 {
		t.Errorf("totals[a] = %d, %v", n, ok)
	}
	if n, ok := b.ByBoardTotals().Get("b1"); !ok || n != 8 {
		t.Errorf("byBoardTotals[b1] = %d, %v", n, ok)
	}
	if n := b.PerTone(p.ToneSeries1); n.Len() != 2 {
		t.Errorf("perTone(series-1) = %v", n)
	}
	if s, ok := b.ByLit().Get("all"); !ok || s.Home() != boards.At(1) {
		t.Errorf("byLit[all] = %v, %v", s, ok)
	}
	if s, ok := b.RankSlots().Get(p.RankHigh); !ok || s.Home() != boards.At(0) {
		t.Errorf("rankSlots[high] = %v, %v", s, ok)
	}
}
