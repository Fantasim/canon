package cppruntime

import (
	"strings"
	"testing"
)

// marksPerText is how many times each runtime text holds the stamp: the check, the guard's define and the namespace's two lines.
const marksPerText = 4

// CODEGEN.md §7.4, DECISIONS 436: each embedded text holds @CANON_STAMP@ four times; the stamp is the first 12 lowercase hex digits of the SHA-256 of both texts, held by the written texts at those four places and nowhere else, and a one-byte change to either text gives another.
func TestStamp(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, text, written, guard string
	}{
		{"canon_runtime.h", headerText, Header, "CANON_RUNTIME_RT_V1"},
		{"canon_runtime_json.h", jsonText, JSON, "CANON_RUNTIME_JSON_RT_V1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if n := strings.Count(c.text, stampMark); n != marksPerText {
				t.Errorf("the embedded text holds %s %d times, want %d", stampMark, n, marksPerText)
			}
			if n := strings.Count(c.written, Stamp); n != marksPerText || strings.Contains(c.written, stampMark) {
				t.Errorf("the written text holds the stamp %d times, want %d, and %s never", n, marksPerText, stampMark)
			}
			for _, place := range []string{
				"\n#if defined(" + c.guard + ") && !defined(" + c.guard + "_" + Stamp + ")\n",
				"\n#define " + c.guard + "_" + Stamp + "\n",
				"\ninline namespace rt_v1_" + Stamp + " {\n",
				"\n}  // namespace rt_v1_" + Stamp + "\n",
			} {
				if strings.Count(c.written, place) != 1 {
					t.Errorf("the written text does not hold %q once", place)
				}
			}
		})
	}
	t.Run("stampOf", func(t *testing.T) {
		t.Parallel()
		if len(Stamp) != stampLen || strings.Trim(Stamp, "0123456789abcdef") != "" {
			t.Errorf("stamp %q is not %d lowercase hexadecimal digits", Stamp, stampLen)
		}
		if got := stampOf(headerText, jsonText); got != Stamp {
			t.Errorf("stampOf(the texts) = %s, Stamp %s", got, Stamp)
		}
		edit := func(s string) string { return s[:len(s)-1] + "\t" }
		if stampOf(edit(headerText), jsonText) == Stamp || stampOf(headerText, edit(jsonText)) == Stamp {
			t.Error("a one-byte change to a runtime text keeps the stamp")
		}
	})
}
