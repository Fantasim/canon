package finding

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeClipsByRune(t *testing.T) {
	long := strings.Repeat("é", maxDetail+10)
	got := Normalize(long)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != maxDetail {
		t.Fatalf("Normalize clipped to %d runes, valid=%v", utf8.RuneCountInString(got), utf8.ValidString(got))
	}
	if Normalize("short") != "short" {
		t.Fatal("a short detail must stay whole")
	}
}
