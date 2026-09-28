package i18n_test

import "testing"

// I18N.md F6: source texts are held in one form (literal braces doubled, interpolations as
// written) whatever the source string's kind: plain, raw or multiline.
func TestSourceTextOneFormPerStringKind(t *testing.T) {
	tests := []struct {
		name, help, want string
	}{
		{"plain", `"Plain {{x}} end"`, "Plain {{x}} end"},
		{"raw", `r"Raw {x} end"`, "Raw {{x}} end"},
		{"multiline", "\"\"\"\nMulti {{x}} end\n\"\"\"", "Multi {{x}} end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := catalogue(t, map[string]string{"st/st.canon": `package st

/// A thing.
record Thing {
  /// Its name.
  name: String
}

view Thing {
  name "Name" { help: ` + tt.help + ` }
}
`}, "st")
			e, ok := cat.Lookup("Thing.name.help")
			if !ok {
				t.Fatalf("missing Thing.name.help in %v", keys(cat))
			}
			if e.Text != tt.want {
				t.Errorf("Text = %q, want %q", e.Text, tt.want)
			}
		})
	}
}
