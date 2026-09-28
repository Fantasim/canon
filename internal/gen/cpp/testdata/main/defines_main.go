// Loads each argument, a hits.json path, with the generated Go demo.defs loader and prints one
// line per argument: the load error, or every row's define refs as key=value (CODEGEN.md §5.8).
// The Go twin of defines_main.cpp (TestDefineParity).
package main

import (
	"fmt"
	"os"
	"strings"

	defs "example.com/parity/defs"
	"example.com/parity/defs/rt"
)

// pairs is keys and values as "k=v" joined by ',', matching defines_main.cpp's Pairs.
func pairs(keys rt.List[string], values rt.List[int64]) string {
	var out []string
	for i := range keys.Len() {
		out = append(out, fmt.Sprintf("%s=%d", keys.At(i), values.At(i)))
	}
	return "[" + strings.Join(out, ",") + "]"
}

func row(h *defs.Hit) string {
	alt := "none"
	if k, ok := h.AltID(); ok {
		v, _ := h.AltValue()
		alt = fmt.Sprintf("%s=%d", k, v)
	}
	maybe := "none"
	if k, ok := h.MaybeIDs(); ok {
		v, _ := h.MaybeValues()
		maybe = pairs(k, v)
	}
	var bonuses []string
	for b := range h.Bonuses().All() {
		bonuses = append(bonuses, fmt.Sprintf("%s=%d:%d", b.KindID(), b.KindValue(), b.Amount()))
	}
	return fmt.Sprintf("%s monster=%s=%d alt=%s all=%s maybe=%s deep=%s=%d bonuses=[%s]", h.ID(), h.MonsterID(), h.MonsterValue(), alt,
		pairs(h.AllIDs(), h.AllValues()), maybe, h.DeepID(), h.DeepValue(), strings.Join(bonuses, ","))
}

func main() {
	for _, path := range os.Args[1:] {
		hits, err := defs.LoadHits(path)
		if err != nil {
			fmt.Println("error " + err.Error())
			continue
		}
		var rows []string
		for h := range hits.All() {
			rows = append(rows, row(h))
		}
		fmt.Println(strings.Join(rows, " | "))
	}
}
