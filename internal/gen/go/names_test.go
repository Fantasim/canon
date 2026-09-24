package gogen

import (
	"errors"
	"testing"
)

// CODEGEN.md §3.2: Go camel case with the closed initialism list (II is none of them; ir.GoWords tests the split itself, decision 120).
func TestCamel(t *testing.T) {
	cases := []struct{ in, upper, lower string }{
		{"id", "ID", "id"},
		{"minRole", "MinRole", "minRole"},
		{"apiKey", "APIKey", "apiKey"},
		{"series_1", "Series1", "series1"},
		{"II_WEA_AXE_ANGEL", "IiWeaAxeAngel", "iiWeaAxeAngel"},
		{"none_", "None", "none"},
		{"url_ts_db", "URLTSDB", "urlTSDB"},
		{"hpMax", "HPMax", "hpMax"},
		{"JSONPath", "JSONPath", "jsonPath"},
		{"_", "", ""},
	}
	for _, c := range cases {
		if got := upperCamel(c.in); got != c.upper {
			t.Errorf("upperCamel(%q) = %q, want %q", c.in, got, c.upper)
		}
		if got := lowerCamel(c.in); got != c.lower {
			t.Errorf("lowerCamel(%q) = %q, want %q", c.in, got, c.lower)
		}
	}
}

// CODEGEN.md §3.4: a reserved name in a lower-case position gets a `_` suffix.
func TestStorageName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"default", "default_"},
		{"type", "type_"},
		{"len", "len_"},
		{"string", "string_"},
		{"rt", "rt_"},
		{"iter", "iter_"},
		{"self", "self_"},
		{"embed", "embed_"},
		{"minRole", "minRole"},
		{"Default", "default_"},
		{"route", "route"},
	}
	for _, c := range cases {
		if got := storageName(c.in); got != c.want {
			t.Errorf("storageName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A first capital on type names; an override is the whole name; the id type and member are
// <Element>ID, <Element>ID<Key> (GoCap on the suffix).
func TestExportedNames(t *testing.T) {
	if got := typeGoName("", "potion"); got != "Potion" {
		t.Errorf("typeGoName = %q", got)
	}
	if got := exportedName("GetID", "id"); got != "GetID" {
		t.Errorf("exportedName = %q", got)
	}
	if got := idTypeName("Status"); got != "StatusID" {
		t.Errorf("id type = %q", got)
	}
	if got := idMemberName(idTypeName("Status"), "wont_do"); got != "StatusIDWontDo" {
		t.Errorf("id member = %q", got)
	}
}

// CODEGEN.md §3.5: two names equal in one scope, or a name that is not an identifier, fail.
func TestScope(t *testing.T) {
	s := newScope("enum Tone")
	if err := s.add("ToneSeries1", "series_1"); err != nil {
		t.Fatal(err)
	}
	if err := s.add("ToneSeries1", "series1"); !errors.Is(err, ErrNameCollision) {
		t.Errorf("collision: %v", err)
	}
	for _, bad := range []string{"", "1abc", "func", "a-b"} {
		if err := s.add(bad, "x"); !errors.Is(err, ErrName) {
			t.Errorf("add(%q) = %v, want ErrName", bad, err)
		}
	}
}
