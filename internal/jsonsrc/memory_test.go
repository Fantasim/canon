package jsonsrc_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// maxBytesPerInputByte bounds what Parse allocates per byte of input: a tree node per value.
const maxBytesPerInputByte = 150

// Parse allocates in proportion to the input: a pointer is built on demand, never stored,
// so a long key or a deep nesting above many values costs nothing per value.
func TestParseMemoryIsLinear(t *testing.T) {
	const n = 20000
	zeros := strings.Repeat("0,", n-1) + "0"
	shapes := []struct{ name, text, last string }{
		{"long key", `{"` + strings.Repeat("k", n) + `": [` + zeros + "]}", "/" + strings.Repeat("k", n) + "/19999"},
		{"deep", strings.Repeat("[", 512) + zeros + strings.Repeat("]", 512), strings.Repeat("/0", 511) + "/19999"},
	}
	for _, sh := range shapes {
		name, text := sh.name, sh.text
		fs := &source.FileSet{}
		f, err := fs.Add("a.json", "/p/a.json", []byte(text))
		if err != nil {
			t.Fatal(err)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		root, err := jsonsrc.Parse(f, diag.NewBag(fs, "p"))
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		perByte := (after.TotalAlloc - before.TotalAlloc) / uint64(len(text))
		t.Logf("%s: %d bytes allocated per input byte", name, perByte)
		if perByte > maxBytesPerInputByte {
			t.Errorf("%s: %d bytes allocated per input byte, want at most %d", name, perByte, maxBytesPerInputByte)
		}
		last := root
		for len(last.Elems)+len(last.Members) > 0 {
			if len(last.Members) > 0 {
				last = last.Members[0].Value
			} else {
				last = last.Elems[len(last.Elems)-1]
			}
		}
		if got := last.Pointer(); got != sh.last {
			t.Errorf("%s: last pointer %.40q..., want %.40q...", name, got, sh.last)
		}
	}
}
