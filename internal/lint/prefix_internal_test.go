package lint

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// API.md §5.7 (rule L2): the suggested name lowers the leading capitals, but the last of a run of two or more before a lower-case letter.
func TestSuggestName(t *testing.T) {
	for _, c := range []struct{ rest, want string }{
		{"ItemCountMax", "itemCountMax"},
		{"ID", "id"},
		{"URLPath", "urlPath"},
		{"IDs", "iDs"},
		{"Log", "log"},
		{"URL2", "url2"},
		{"X", "x"},
		{"value", "value"},
		{"Ab", "ab"},
		{"ABc", "aBc"},
	} {
		if got := suggestName(c.rest); got != c.want {
			t.Errorf("suggestName(%q) = %q, want %q", c.rest, got, c.want)
		}
	}
}

// API.md §5.7 (rule L2): which prefix a name keeps, by the kind of the field's type.
func TestMatchPrefix(t *testing.T) {
	for _, c := range []struct {
		name         string
		kind         types.Kind
		prefix, rest string
		ok           bool
	}{
		{"dwItemCount", types.Int, "dw", "ItemCount", true},
		{"dwItemCount", types.List, "dw", "ItemCount", true},
		{"szName", types.String, "sz", "Name", true},
		{"lpPoint", types.Record, "lp", "Point", true},
		{"strLabel", types.Float, "str", "Label", true},
		{"nLog", types.Int, "n", "Log", true},
		{"nLog", types.Enum, "n", "Log", true},
		{"nLog", types.Ref, "n", "Log", true},
		{"nLog", types.String, "", "", false},
		{"wDps", types.Float, "", "", false},
		{"iCount", types.Int, "i", "Count", true},
		{"uMask", types.Int, "u", "Mask", true},
		{"fScale", types.Float, "f", "Scale", true},
		{"fScale", types.Int, "", "", false},
		{"bOn", types.Bool, "b", "On", true},
		{"bFlags", types.Int, "b", "Flags", true},
		{"bText", types.String, "", "", false},
		{"m_value", types.Int, "m_", "value", true},
		{"m_Value", types.String, "m_", "Value", true},
		{"m_1", types.Int, "", "", false},
		{"dwlower", types.Int, "", "", false},
		{"dw", types.Int, "", "", false},
		{"dw_x", types.Int, "", "", false},
		{"clean", types.Int, "", "", false},
		{"number", types.Int, "", "", false},
	} {
		prefix, rest, ok := matchPrefix(c.name, c.kind)
		if prefix != c.prefix || rest != c.rest || ok != c.ok {
			t.Errorf("matchPrefix(%q, %v) = %q, %q, %t; want %q, %q, %t", c.name, c.kind, prefix, rest, ok, c.prefix, c.rest, c.ok)
		}
	}
}
