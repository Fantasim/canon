package load

import (
	"bytes"
	"slices"
	"testing"
)

// crlf and lf are the FileSet's folding (source.FileSet.Add): each CR LF becomes one LF.
var crlf, lf = []byte("\r\n"), []byte("\n")

// WIRE.md §6.6: foldCursor agrees with the FileSet's folding at every offset, asked in any order.
func TestFoldCursorMatchesFileSetFolding(t *testing.T) {
	for _, data := range []string{"", "a", "\r", "\r\n", "a\r\nb", "\r\r\n\n", "\r\n\r\n", "a\rb\r\n", "\n\r"} {
		raw := []byte(data)
		c := foldCursor{data: raw}
		var offsets []int
		for i := 0; i <= len(raw); i++ {
			offsets = append(offsets, i)
		}
		back := slices.Clone(offsets)
		slices.Reverse(back)
		for _, i := range slices.Concat(offsets, back, offsets, []int{len(raw) + 1, -1}) {
			j := min(max(i, 0), len(raw))
			want := len(bytes.ReplaceAll(raw[:j], crlf, lf))
			if got := c.at(i); got != want {
				t.Errorf("%q: at(%d) = %d, want %d", data, i, got, want)
			}
		}
	}
}
