package edit

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
)

// API.md E34 (WIRE.md 5.5.2): the default wire name a renamed field's converse compares with,
// for every row of WIRE.md's table.
func TestRenameWireName(t *testing.T) {
	cases := []struct{ name, snake, kebab, upper string }{
		{"targetTime", "target_time", "target-time", "TARGET_TIME"},
		{"maxQuestsPerStyle", "max_quests_per_style", "max-quests-per-style", "MAX_QUESTS_PER_STYLE"},
		{"ratesBySpecific", "rates_by_specific", "rates-by-specific", "RATES_BY_SPECIFIC"},
		{"isDailyFlat", "is_daily_flat", "is-daily-flat", "IS_DAILY_FLAT"},
		{"stage1Rate", "stage1_rate", "stage1-rate", "STAGE1_RATE"},
		{"level2", "level2", "level2", "LEVEL2"},
		{"v2beta", "v2beta", "v2beta", "V2BETA"},
		{"HTTPServer", "http_server", "http-server", "HTTP_SERVER"},
		{"userID", "user_id", "user-id", "USER_ID"},
		{"__max__players", "max_players", "max-players", "MAX_PLAYERS"},
	}
	for _, c := range cases {
		got := []string{check.ConvertCase(c.name, "snake"), check.ConvertCase(c.name, "kebab"), check.ConvertCase(c.name, "upper_snake")}
		if want := []string{c.snake, c.kebab, c.upper}; !slices.Equal(got, want) {
			t.Errorf("WIRE.md §5.5.2, %s: %v, want %v", c.name, got, want)
		}
		if check.ConvertCase(c.name, "camel") != c.name || check.ConvertCase(c.name, "") != c.name {
			t.Errorf("WIRE.md §5.5.2, %s: camel or no case changes the name", c.name)
		}
	}
}

// API.md E27: a position is `file:line:col`, both positive decimals; anything else is a names path.
func TestRenameSplitPosition(t *testing.T) {
	cases := []struct {
		in   string
		want namePos
		ok   bool
	}{
		{"a/b.canon:3:14", namePos{file: "a/b.canon", line: 3, col: 14}, true},
		{"@root/x.canon:1:1", namePos{file: "@root/x.canon", line: 1, col: 1}, true},
		{"pkg:Type.field", namePos{}, false},
		{"a.canon:0:1", namePos{}, false},
		{"a.canon:1:+2", namePos{}, false},
		{":1:2", namePos{}, false},
		{"a.canon:1", namePos{}, false},
	}
	for _, c := range cases {
		got, ok := splitPosition(c.in)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("API.md E27, %q: %+v %v, want %+v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// API.md E31: RenameRequest refuses a RenameName beside another op or with AllowErrors as
// ErrBadOp, under an edit layer as reason layer, at the RenameName's index.
func TestRenameRequestRule(t *testing.T) {
	rn := Operation{Kind: OpRenameName, Path: "a:x", Name: "y"}
	set := Operation{Kind: OpSet, Path: "a:v", Value: Int(1)}
	cases := []struct {
		ops     []Operation
		errs    bool
		layer   string
		want    error
		atIndex int
	}{
		{[]Operation{rn}, false, "", nil, 0},
		{[]Operation{set}, true, "dev", nil, 0},
		{[]Operation{set, rn}, false, "", ErrBadOp, 1},
		{[]Operation{rn}, true, "", ErrBadOp, 0},
		{[]Operation{rn}, false, "dev", ErrNotEditable, 0},
	}
	for i, c := range cases {
		err := RenameRequest(c.ops, c.errs, c.layer)
		var oe *OpError
		switch {
		case c.want == nil && err != nil:
			t.Errorf("API.md E31, case %d: %v", i, err)
		case c.want != nil && (!errors.Is(err, c.want) || !errors.As(err, &oe) || oe.Index != c.atIndex):
			t.Errorf("API.md E31, case %d: %v, want %v at %d", i, err, c.want, c.atIndex)
		}
	}
}

// API.md E24, API.md E30: the JSON form of RenameName writes and reads `name`, an empty one
// included (E30 refuses it when the op applies); one without `name` does not decode.
func TestRenameOpJSON(t *testing.T) {
	op := Operation{Kind: OpRenameName, Path: "features/a.canon:3:5", Name: "hp"}
	data, err := json.Marshal(op)
	if want := `{"op":"renameName","path":"features/a.canon:3:5","name":"hp"}`; err != nil || string(data) != want {
		t.Fatalf("API.md E24: %s, %v", data, err)
	}
	var back Operation
	if err := json.Unmarshal(data, &back); err != nil || back != op {
		t.Errorf("API.md E24: %+v, %v", back, err)
	}
	empty := Operation{Kind: OpRenameName, Path: "a:b"}
	if err := json.Unmarshal([]byte(`{"op":"renameName","path":"a:b","name":""}`), &back); err != nil || back != empty {
		t.Errorf("API.md E30: an empty name does not decode: %+v, %v", back, err)
	}
	if data, err := json.Marshal(empty); err != nil || string(data) != `{"op":"renameName","path":"a:b","name":""}` {
		t.Errorf("API.md E24: an empty name encodes as %s, %v", data, err)
	}
	if err := json.Unmarshal([]byte(`{"op":"renameName","path":"a:b"}`), &back); !errors.Is(err, ErrOpJSON) {
		t.Errorf("API.md E24: a RenameName without a name decoded: %v", err)
	}
}
