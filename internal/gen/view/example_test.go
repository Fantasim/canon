package viewgen_test

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

// An integer beyond ±(2^53-1) written as a decimal string (VIEWMODEL.md J10).
func Example() {
	big := vm.Number{Text: "9007199254740993", Quoted: true}
	field := vm.Field{Name: "big", Type: vm.TypeExpr{Kind: "int", Bits: 64, Min: big}, Wire: vm.Wire{Name: "big"}}
	m := &vm.ViewModel{
		Schema: "canon-vm/1", Package: "demo", Language: "0.1",
		Types: map[string]vm.TypeDef{"demo.T": {Kind: "record", Name: "T", Fields: []vm.Field{field}}},
		I18N:  vm.I18N{Source: "en", Languages: map[string]vm.Language{}},
	}
	out, err := viewgen.Write(m)
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, big.Text) {
			fmt.Println(strings.TrimSpace(l), err)
		}
	}
	// Output: "min": "9007199254740993" <nil>
}
