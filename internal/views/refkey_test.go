package views_test

import (
	"testing"

	"github.com/fantasim/canonlang/api/vm"
)

// refChain is keyed lists chained by ref keys to an Int key, and a record with a ref into each.
const refChain = `package a

local record Item {
  id: Int
}

local record Mid {
  item: ref items
}

local record Top {
  mid: ref mids
}

local let items: [Item] keyed by id = [{ id: 7 }]
local let mids: [Mid] keyed by item = [{ item: 7 }]
local let tops: [Top] keyed by mid = [{ mid: 7 }]

record Holder {
  one: ref items
  two: ref mids?
  three: ref tops?
}

view Holder {
  title "Holder"
}

let holders: table Holder = {
  h { one: 7, two: 7, three: 7 }
}
`

// VIEWMODEL.md J10, J12, DECISIONS 432: a ref whose target is keyed by a ref takes its key type
// from the chain's leaf key at every depth, as its entry keys are written (J10), a JSON number
// for an Int leaf, the third level's row key included; one level is unchanged.
func TestRefKeyTypeFollowsChain(t *testing.T) {
	m := demo(t, refChain, "").model(t, demoPkg)
	for i, coll := range []string{"a:items", "a:mids", "a:tops"} {
		if got := m.Search[coll].KeyType; got != "int" {
			t.Errorf("J12 %s: search keyType %q, want int", coll, got)
		}
		if rows := m.Search[coll].Rows; len(rows) != 1 || rows[0].Key != vm.Scalar(vm.Int(7)) {
			t.Errorf("J10 %s: written rows %+v, want one row keyed by the integer 7", coll, rows)
		}
		ref := m.Types["a.Holder"].Fields[i].Type
		if ref.Of != nil {
			ref = *ref.Of // the optional's
		}
		if got := ref.KeyType; got != "int" {
			t.Errorf("J12 %s: ref keyType %q, want int", coll, got)
		}
	}
}
