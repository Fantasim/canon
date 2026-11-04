package typedef_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/views/typedef"
)

// computedSource has defaults reading the instance by bare name, through `self` and through a
// parameter, in a record, an applied record and a case body, beside defaults that read none.
const computedSource = `package a

record Server {
  port: Int = 80
  byName: Int = port * 2
  bySelf: Int = self.port * 2
  nameLabel: String = "{port}"
  selfLabel: String = "{self.port}"
  plain: Int = 3
}

record Ev {
  max: Int = 3
}

record Lim(e: Ev) {
  lim: Int = e.max
  label: String = "{self.lim}"
  twice: Int = self.lim * 2
}

variant V {
  c {
    n: Int = 1
    byName: String = "{n}"
    bySelf: String = "{self.n}"
  }
}
`

// computedDefault is the view model's marker of a default that reads the instance.
const computedDefault = `{"computed":true}`

// VIEWMODEL.md Field, J10; TYPES.md §3.3, §11.1, §15: a default reading `self.f` is computed as one naming f bare.
func TestComputedDefaults(t *testing.T) {
	section := typedef.New(context.Background(), typedef.Input{Program: checked(t, computedSource), Fold: noFold{}}, "a").Section()
	for _, c := range []struct {
		typ, kase string
		computed  []string
	}{
		{"a.Server", "", []string{"byName", "bySelf", "nameLabel", "selfLabel"}},
		{"a.Ev", "", nil},
		{"a.Lim", "", []string{"lim", "label", "twice"}},
		{"a.V", "c", []string{"byName", "bySelf"}},
	} {
		def, ok := section[c.typ]
		if !ok {
			t.Fatalf("%s is not in the section", c.typ)
		}
		var got []string
		for _, f := range fieldsOf(def, c.kase) {
			if string(f.Default) == computedDefault {
				got = append(got, f.Name)
			}
		}
		if !slices.Equal(got, c.computed) {
			t.Errorf("%s %s: computed defaults %v, want %v", c.typ, c.kase, got, c.computed)
		}
	}
}

// fieldsOf are a record's fields, or those of the variant's case named kase.
func fieldsOf(def vm.TypeDef, kase string) []vm.Field {
	if kase == "" {
		return def.Fields
	}
	for _, c := range def.Cases {
		if c.Name == kase {
			return c.Fields
		}
	}
	return nil
}
