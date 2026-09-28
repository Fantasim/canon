package vm_test

import (
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
)

// A field of a record type as internal/views builds it: the bound beyond 2^53−1 is a decimal
// string (VIEWMODEL.md J10), the help a text key (J9).
func Example() {
	signed := true
	f := vm.Field{
		Name:     "heal",
		Type:     vm.TypeExpr{Kind: "int", Bits: 64, Signed: &signed, Min: vm.Int(1), Max: vm.Int(1 << 60)},
		Required: true,
		Help:     vm.TextRef{Key: "pipeline:Potion.heal.help"},
		Wire:     vm.Wire{Name: "nHeal"},
	}
	out, err := json.Marshal(f)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(out))

	var back vm.Field
	if err := json.Unmarshal(out, &back); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(back.Type.Max.Text, back.Type.Max.Quoted)
	// Output:
	// {"name":"heal","type":{"kind":"int","bits":64,"signed":true,"min":1,"max":"1152921504606846976"},"required":true,"help":"pipeline:Potion.heal.help","wire":{"name":"nHeal"}}
	// 1152921504606846976 true
}

// An enum member whose wire value is a number (@json(codes)) and a language-neutral label.
func ExampleScalar() {
	label := "—"
	m := vm.Member{Name: "NONE", Wire: vm.Scalar{Text: "0"}, Index: 0, Label: vm.TextRef{Text: &label}}
	out, err := json.Marshal(m)
	fmt.Println(string(out), err)
	// Output: {"name":"NONE","wire":0,"index":0,"label":{"text":"—"}} <nil>
}
