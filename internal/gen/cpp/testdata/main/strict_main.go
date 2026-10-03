// Loads each argument, <kind>[:<locale>]=<path>, with the generated Go demo.shop and demo.nest
// loaders: "shelves" a file through LoadShelves, "reload" a directory through Store.Reload,
// "nested" a file through nest.LoadValue; the <locale> is strict_main.cpp's alone (Go reads no
// process locale). Prints one line per argument: the load error, or a canonical dump of the
// loaded Int/Float/Float32 fields (every one, optionals included), so the Go and C++ loaders are
// compared on values, not only on refusals (log-2026-09-24 loader parity). The Go twin of
// strict_main.cpp (TestLoaderParity).
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	nest "example.com/parity/nest"
	shop "example.com/parity/shop"
)

// signedFloat is the sign bit, then the magnitude at prec significant digits ("+12.5", "-0"),
// matching strict_main.cpp's SignedFloat.
func signedFloat(v float64, prec int) string {
	sign := "+"
	if math.Signbit(v) {
		sign = "-"
	}
	return sign + strconv.FormatFloat(math.Abs(v), 'g', prec, 64)
}

// bits is a Float32's bits in hex ("3f800001"), matching strict_main.cpp's Bits.
func bits(f float32) string {
	return fmt.Sprintf("%08x", math.Float32bits(f))
}

// dumpDrawers lists a nested table's rows in order, "id:n" and an "r" after a retired one, matching strict_main.cpp's DumpDrawers.
func dumpDrawers(n int, at func(int) *nest.Drawer) string {
	var out []string
	for i := range n {
		r := at(i)
		retired := ""
		if r.Retired() {
			retired = "r"
		}
		out = append(out, string(r.ID())+":"+strconv.FormatInt(r.N(), 10)+retired)
	}
	return strings.Join(out, ",")
}

// dumpHolder lists every Float32 of demo.nest's Holder as bits, matching strict_main.cpp's
// DumpHolder.
func dumpHolder(h *nest.Holder) string {
	var f32s []string
	for f := range h.F32S().All() {
		f32s = append(f32s, bits(f))
	}
	r := "none"
	if c, ok := h.Shape().AsCircle(); ok {
		r = bits(c.R())
	}
	spare := "none"
	if rows, ok := h.Spare(); ok {
		spare = dumpDrawers(rows.Len(), rows.At)
	}
	return "f32s=" + strings.Join(f32s, ",") + " drawers=" + dumpDrawers(h.Drawers().Len(), h.Drawers().At) + " spare=" + spare + " fav=" + string(h.FavID()) + " a.b=" + bits(h.Dotted()) + " a/b=" + bits(h.Deep()) +
		" shape.r=" + r + " $scale=" + bits(h.Scale(nest.SizeSmall)) + "," + bits(h.Scale(nest.SizeMedium))
}

// optInt and optFloat are "none" for an absent optional, else the value (log-2026-09-24).
func optInt(v int64, ok bool) string {
	if !ok {
		return "none"
	}
	return strconv.FormatInt(v, 10)
}

func optFloat(v float64, ok bool, prec int) string {
	if !ok {
		return "none"
	}
	return signedFloat(v, prec)
}

// dumpShelves lists id:slots=,delete=,i= for every row, matching strict_main.cpp's DumpShelves.
func dumpShelves(s *shop.Shelves) string {
	var parts []string
	for r := range s.All() {
		parts = append(parts, r.ID()+":slots="+strconv.FormatInt(int64(r.Slots()), 10)+
			",delete="+strconv.FormatInt(r.Delete(), 10)+",i="+strconv.FormatInt(r.I(), 10))
	}
	return strings.Join(parts, " ")
}

// dumpItems lists id:code=,price=,weight=,bonus=,legacyMax= for every row, matching
// strict_main.cpp's DumpItems.
func dumpItems(s *shop.Items) string {
	var parts []string
	for r := range s.All() {
		bonus, bonusOK := r.Bonus()
		parts = append(parts, string(r.ID())+":code="+strconv.FormatUint(uint64(r.Code()), 10)+
			",price="+signedFloat(r.Price(), 17)+",weight="+signedFloat(float64(r.Weight()), 9)+
			",bonus="+optInt(bonus, bonusOK)+",legacyMax="+strconv.FormatInt(r.LegacyMax(), 10))
	}
	return strings.Join(parts, " ")
}

// dumpConfig is "config:scale=", matching strict_main.cpp's DumpConfig.
func dumpConfig(c *shop.Config) string {
	scale, ok := c.Scale()
	return "config:scale=" + optFloat(scale, ok, 17)
}

func main() {
	for _, arg := range os.Args[1:] {
		kind, path, ok := strings.Cut(arg, "=")
		kind, _, _ = strings.Cut(kind, ":")
		var err error
		out := ""
		switch {
		case ok && kind == "shelves":
			var s *shop.Shelves
			if s, err = shop.LoadShelves(path); err == nil {
				out = dumpShelves(s)
			}
		case ok && kind == "reload":
			if err = shop.Store.Reload(path); err == nil {
				snap := shop.Store.Current()
				out = dumpItems(snap.Items()) + " " + dumpConfig(snap.Config())
			}
		case ok && kind == "nested":
			var h *nest.Holder
			if h, err = nest.LoadValue(path); err == nil {
				out = dumpHolder(h)
			}
		default:
			os.Exit(100)
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(out)
	}
}
