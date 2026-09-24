package constructs_test

import (
	"fmt"

	constructs "example.com/features/constructs/out/go"
)

// Smoke test of the generated constructs package.
func Example() {
	fmt.Println(constructs.MaxHeal, constructs.Rate, constructs.SmallRate, constructs.Greeting, constructs.Enabled)
	fmt.Println(constructs.Tick, constructs.NoWait, constructs.DefaultGrade, constructs.Primes().Clone())
	for k, v := range constructs.Weights().All() {
		fmt.Println(k, v)
	}
	fmt.Println(constructs.ElementFire.Code(), constructs.ElementEarth, constructs.GradeUltimate1.Wire(), constructs.FeelingAPIURL)
	e, ok := constructs.ElementFromCode(4)
	_, bad := constructs.ElementFromCode(3)
	fmt.Println(e, ok, bad, constructs.Element(9), constructs.GradeUnique < constructs.GradeUltimate1)
	for p := range constructs.GetPotions().All() {
		note, hasNote := p.Note()
		level, hasLevel := p.Level()
		parent, hasParent := p.ParentID()
		fmt.Println(p.ID(), p.Heal(), p.Cooldown(), p.Ratio(), p.Weight(), p.Element(), p.Tags().Clone(), note, hasNote, level, hasLevel, parent, hasParent)
		fmt.Println(p.Shape().Kind(), p.Extra() != nil, p.Default(), p.APIKey(), p.HitPoints(), p.RunesIDs().Clone(), p.Grades().Len())
		c, _ := p.Shape().AsCircle()
		s, _ := p.Shape().AsSquare()
		fmt.Println(c != nil, s != nil, p.IsStrong(), p.CanUse(constructs.GradeUnique, false), p.CanUse(constructs.GradeUltimate1, true))
		cost, hasCost := p.CostIn(constructs.ElementEarth)
		fmt.Println(cost, hasCost)
	}
	large, _ := constructs.GetPotions().Find("large")
	fmt.Println(large.Parent().Heal(), constructs.GetFavorite() == large, constructs.GetFavoriteID(), large.Extra().Kind())
	r, _ := constructs.GetRunes().FindByCode(20)
	fmt.Println(r.ID(), r.Retired(), r.Mood(), constructs.GetRunes().Len())
	_, some := constructs.GetMaybeRuneID()
	fmt.Println(constructs.GetMaybeRune() == nil, some, constructs.GetNames().Clone(), constructs.GetZero(), constructs.GetZero32(), constructs.GetMoods().Clone())
	hp, _ := constructs.GetLimits().Get("hp")
	delay, _ := constructs.GetDelay()
	sq, _ := constructs.GetOrigin().AsSquare()
	label, _ := sq.Label()
	fmt.Println(hp, delay, sq.Side(), label)
	fmt.Println(constructs.UpgradeCost(constructs.GradeUltimate1), constructs.CanUpgrade(constructs.GradeUnique, true), constructs.Answer())
	fmt.Println(constructs.RuneFor(constructs.ElementWater) == nil, constructs.RuneFor(constructs.ElementEarth).Label(), constructs.BestPotion().ID())
	b, okB := constructs.BonusOf(constructs.RuneIDAlpha)
	_, okC := constructs.BonusOf(constructs.RuneIDBeta2)
	fmt.Println(b, okB, okC, constructs.ShapeOf(constructs.GradeNormal).Kind(), constructs.ShapeOf(constructs.GradeUnique) == nil)
	// Output:
	// 9000 0.25 0.1 héllo
	// "world" true
	// 1m30s 0s unique [2 3 5 7]
	// a 1.5
	// b -2
	// 1 EARTH ultimate-1 api_url
	// EARTH true false Element(9) true
	// small 50 1.5s 0.5 1.25 FIRE [a b]  false 0 false  false
	// circle false false  1 [alpha] 2
	// true false false true false
	// 5 true
	// large 500 3s -0 2 WATER [] big true -3 true small true
	// square true true k 18446744073709551 [beta_2 alpha] 0
	// false true true true true
	// 0 false
	// 50 true large none_
	// beta_2 true api_url 2
	// true false [x] -0 -0 [api_url]
	// 100 1.5s 1 o
	// 20000000 true 42
	// true Beta small
	// 7 true false circle true
}
