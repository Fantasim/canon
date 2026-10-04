// Package smoke is copied beside a temporary copy of examples/telemetry/expected/services by
// `make goldens-vet` (DECISIONS 201): it never lives in expected/, which holds only compiler
// output. It reads examples/telemetry's baked Go as monitoring would.
package smoke

import (
	"testing"

	"gitlab.com/sovereign15/monitoring/gen/telemetry"
)

// CODEGEN.md §5.10: a lookup over an enum is a dense table over every member, retired ones
// included (domain order); §5.2: a @codes enum's value is its code.
func TestLookups(t *testing.T) {
	for _, c := range []struct {
		e        telemetry.EventType
		code     uint8
		version  uint8
		tier     telemetry.Tier
		size     int64
		instance bool
		retired  bool
	}{
		{telemetry.EventTypeItemCreated, 11, 5, telemetry.TierWarm, 38, true, false},
		{telemetry.EventTypeMailItemsReceived, 20, 0, telemetry.TierReadonly, 0, false, true},
		{telemetry.EventTypeItemConsumed, 31, 3, telemetry.TierReadonly, 24, true, false},
		{telemetry.EventTypePenyaDrops, 70, 2, telemetry.TierHot, 24, false, false},
		{telemetry.EventTypeTradePenya, 71, 2, telemetry.TierCool, 36, false, false},
		{telemetry.EventTypeServerLifecycle, 110, 1, telemetry.TierTiny, 13, false, false},
		{telemetry.EventTypeFarmEvents, 151, 2, telemetry.TierCool, 37, false, false},
		{telemetry.EventTypeSovereignEvents, 152, 1, telemetry.TierTiny, 40, false, false},
		{telemetry.EventTypeWorldBossSpawns, 156, 0, telemetry.TierReadonly, 0, false, true},
	} {
		if c.e.Code() != c.code || telemetry.VersionOf(c.e) != c.version || telemetry.TierOf(c.e) != c.tier ||
			telemetry.WireSize(c.e) != c.size || telemetry.HasInstanceID(c.e) != c.instance ||
			telemetry.IsRetired(c.e) != c.retired || telemetry.TableOf(c.e) != c.e.String() {
			t.Errorf("%s: code %d v%d %s size %d instance %t retired %t", c.e, c.e.Code(), telemetry.VersionOf(c.e),
				telemetry.TierOf(c.e), telemetry.WireSize(c.e), telemetry.HasInstanceID(c.e), telemetry.IsRetired(c.e))
		}
	}
	if e, ok := telemetry.EventTypeFromCode(156); !ok || e != telemetry.EventTypeWorldBossSpawns {
		t.Errorf("EventTypeFromCode(156) = %v, %t: a tombstone still decodes", e, ok)
	}
	if !telemetry.Emits(telemetry.EventTypeServerLifecycle, telemetry.ProducerLoginServer) ||
		telemetry.Emits(telemetry.EventTypeSovereignEvents, telemetry.ProducerWorldServer) {
		t.Error("Emits: the spine is everywhere, sovereign_events is CoreServer only")
	}
}

// The baked values hold the catalogue: farm_events' retired column and its bounded ledger
// role (CODEGEN.md §5.9).
func TestEvents(t *testing.T) {
	farm, ok := telemetry.GetEvents().Get(telemetry.EventTypeFarmEvents)
	if !ok {
		t.Fatal("no farm_events")
	}
	cost, ok := farm.Columns().Find("cost")
	if !ok {
		t.Fatal("no cost column")
	}
	if in, ok := cost.RetiredIn(); !ok || in != 2 {
		t.Errorf("cost.RetiredIn() = %d, %t, want 2", in, ok)
	}
	role := farm.Ledger().At(0)
	if until, ok := role.UntilVersion(); !ok || until != 1 || role.Token() != "farm" || role.Counted() {
		t.Errorf("farm ledger role: until %d %t, token %s, counted %t", until, ok, role.Token(), role.Counted())
	}
	if amount, ok := role.AmountID(); !ok || amount != "cost" {
		t.Errorf("farm ledger amount: %q %t", amount, ok)
	}
	if telemetry.GetEvents().Len() != 7 {
		t.Errorf("%d live events, want 7", telemetry.GetEvents().Len())
	}
}
