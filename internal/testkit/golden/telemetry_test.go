package golden

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// telemetryTests is how many test blocks examples/telemetry/checks.canon holds.
const telemetryTests = 5

// SPEC §18: telemetry's test blocks pass, pinning coverage (L-0111) and the V2–V4 history checks on broken tables.
func TestTelemetryCanonTests(t *testing.T) {
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, p := openExamples(t, root)
	res, err := p.Test(context.Background(), canon.TestOptions{Packages: []string{"telemetry"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 0 || res.Passed != telemetryTests {
		t.Errorf("canon test telemetry: %d passed, %d failed, want %d passed: %+v", res.Passed, res.Failed, telemetryTests, res.Tests)
	}
}

// telemetryText is where examples/telemetry's `emit text` goldens live.
var telemetryText = filepath.Join(examplesDir, "telemetry", "expected", "source", "Tools", "telemetry", "generated")

// farmEventsDDL is the lake's farm_events DDL today (Source CreateTableSql, via meta/handoff/2026-10-04-telemetry-recon.md).
const farmEventsDDL = "CREATE TABLE IF NOT EXISTS farm_events (event_version UTINYINT NOT NULL, producer_id UINTEGER NOT NULL, " +
	"boot_id BIGINT NOT NULL, batch_seq BIGINT NOT NULL, row_no USMALLINT NOT NULL, timestamp_ns BIGINT NOT NULL, " +
	"player_id UINTEGER NOT NULL, kind UTINYINT, slot_index UINTEGER, type_id UINTEGER, item_count UINTEGER, " +
	"target_player_id UINTEGER, fact_id UBIGINT, cost BIGINT);"

// telemetryDriverOut is what testdata/cpp/telemetry_main.cpp prints: every EventType member
// in declaration order with its constexpr metadata, then farm_events' baked data and DDL.
var telemetryDriverOut = strings.Join([]string{
	"11 item_created v5 WARM size=38 retired=0",
	"20 mail_item_received v0 READONLY size=0 retired=1",
	"31 item_consumed v3 READONLY size=24 retired=0",
	"70 penya_drops v2 HOT size=24 retired=0",
	"71 trade_penya v2 COOL size=36 retired=0",
	"110 server_lifecycle v1 TINY size=13 retired=0",
	"151 farm_events v2 COOL size=37 retired=0",
	"152 sovereign_events v1 TINY size=40 retired=0",
	"156 world_boss_spawns v0 READONLY size=0 retired=1",
	"farm_events columns=7 cost.retiredIn=2 ledger=farm until=1 slots=5",
	farmEventsDDL,
	"",
}, "\n")

// DECISIONS 293, CODEGEN.md §5.10: telemetry's C++ baked lookups serve static_assert, templates, array bounds, if constexpr; -Werror, every toolchain.
func TestTelemetryCppCompiles(t *testing.T) {
	_, roots := buildCpp(t, "telemetry")
	gen := filepath.Join(roots["source"], "Engine", "Telemetry", "Generated")
	driver, err := os.ReadFile(filepath.Join(cppCompileDir, "telemetry_main.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gen, "main.cpp"), driver, filePerm); err != nil {
		t.Fatal(err)
	}
	for _, out := range buildAndRun(t, gen, []string{"main.cpp", "telemetry.gen.cpp"}) {
		if out != telemetryDriverOut {
			t.Errorf("driver output:\n%s\nwant:\n%s", out, telemetryDriverOut)
		}
	}
}

// DECISIONS 294, CODEGEN.md §2.9: tables.sql's farm_events line is the lake's DDL byte for byte, retired column last.
func TestTelemetryDDLMatchesLake(t *testing.T) {
	sql, err := os.ReadFile(filepath.Join(telemetryText, "tables.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(sql), "\n") {
		if strings.HasPrefix(line, "CREATE TABLE IF NOT EXISTS farm_events ") {
			if line != farmEventsDDL {
				t.Errorf("farm_events DDL:\n%s\nwant:\n%s", line, farmEventsDDL)
			}
			return
		}
	}
	t.Error("tables.sql has no farm_events table")
}

// telemetryCatalog is the shape examples/telemetry/catalog.canon documents for catalog.json.
type telemetryCatalog struct {
	Format     int `json:"format"`
	Contract   int `json:"contract"`
	Provenance []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"provenance"`
	Events []telemetryCatalogEvent `json:"events"`
	Enums  []struct {
		Name   string `json:"name"`
		View   string `json:"view"`
		Values []struct {
			Code    int    `json:"code"`
			Name    string `json:"name"`
			Retired bool   `json:"retired"`
		} `json:"values"`
	} `json:"enums"`
}

type telemetryCatalogEvent struct {
	ID            int      `json:"id"`
	Table         string   `json:"table"`
	Retired       bool     `json:"retired"`
	Version       int      `json:"version"`
	Since         int      `json:"since"`
	Tier          string   `json:"tier"`
	Producers     []string `json:"producers"`
	Enqueue       string   `json:"enqueue"`
	HasInstanceID bool     `json:"hasInstanceId"`
	WireSize      int      `json:"wireSize"`
	Description   string   `json:"description"`
	Columns       []struct {
		Name      string  `json:"name"`
		Type      string  `json:"type"`
		Kind      string  `json:"kind"`
		Nullable  bool    `json:"nullable"`
		Since     int     `json:"since"`
		RetiredIn *int    `json:"retiredIn"`
		Enum      *string `json:"enum"`
	} `json:"columns"`
	Ledger []telemetryCatalogRole `json:"ledger"`
}

type telemetryCatalogRole struct {
	Kinds *struct {
		Column string   `json:"column"`
		Values []string `json:"values"`
	} `json:"kinds"`
	Currency     string  `json:"currency"`
	Bucket       string  `json:"bucket"`
	Sign         int     `json:"sign"`
	Token        string  `json:"token"`
	Leg          *string `json:"leg"`
	Filter       *string `json:"filter"`
	Amount       *string `json:"amount"`
	Item         *string `json:"item"`
	Player       string  `json:"player"`
	Counterparty *string `json:"counterparty"`
	Counted      bool    `json:"counted"`
	DedupOf      *string `json:"dedupOf"`
	UntilVersion *int    `json:"untilVersion"`
	Note         string  `json:"note"`
}

// DECISIONS 299: catalog.json parses strictly into its documented shape and lists every event type in id order, tombstones flagged.
func TestTelemetryCatalogParses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(telemetryText, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c telemetryCatalog
	if err := dec.Decode(&c); err != nil {
		t.Fatal(err)
	}
	var ids, tombstones []int
	for _, e := range c.Events {
		ids = append(ids, e.ID)
		if e.Retired {
			tombstones = append(tombstones, e.ID)
		}
	}
	if want := []int{11, 20, 31, 70, 71, 110, 151, 152, 156}; !slices.Equal(ids, want) {
		t.Errorf("event ids %v, want %v", ids, want)
	}
	if want := []int{20, 156}; !slices.Equal(tombstones, want) {
		t.Errorf("tombstones %v, want %v", tombstones, want)
	}
	farm := c.Events[len(c.Events)-3]
	cost := farm.Columns[len(farm.Columns)-2]
	if farm.Table != "farm_events" || cost.Name != "cost" || cost.RetiredIn == nil || *cost.RetiredIn != 2 {
		t.Errorf("farm_events: %s, column %s retiredIn %v", farm.Table, cost.Name, cost.RetiredIn)
	}
	labelled := false
	for _, e := range c.Enums {
		for _, v := range e.Values {
			labelled = labelled || (e.Name == "GrantKind" && v.Code == 3 && v.Name == "LEVEL_UP_GIFT" && v.Retired)
		}
	}
	if !labelled {
		t.Error("GrantKind 3 (retired) has lost its label")
	}
	if len(c.Provenance) != 5 || c.Format != 2 || len(c.Enums) == 0 {
		t.Errorf("catalog: %d provenance columns, format %d, %d enums", len(c.Provenance), c.Format, len(c.Enums))
	}
}
