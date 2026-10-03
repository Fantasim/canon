package lsp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/tools/txtar"
)

// IMPLEMENTATION-PLAN §8.4 Features: a value's hover text is cut to 4,096 bytes, on a character.
func TestHoverCap(t *testing.T) {
	// One line of each list is its whole canonical text: only the byte cap can cut it.
	ints, words := make([]string, 2000), make([]string, 1000)
	for i := range ints {
		ints[i] = strconv.Itoa(i)
	}
	for i := range words {
		words[i] = strconv.Quote("é中😀x") // 14 bytes with ", ": byte 4,096 falls inside an emoji
	}
	law := "-- proj/project.canon --\nproject acme {\n  canon: \"0.1\"\n}\n-- proj/a/a.canon --\n/// A.\npackage a\n\n" +
		"/// Ints.\nlet ints: [Int] = [" + strings.Join(ints, ", ") + "]\n\n" +
		"/// Words.\nlet words: [String] = [" + strings.Join(words, ", ") + "]\n"
	s := newSession(t, txtar.Parse([]byte(law)))
	done := s.start()
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/a/a.canon"}))
	must(t, s.wait(nil)) // the pass opens the project (DECISIONS 285)
	for id, line := range map[int]int{1: 4, 2: 7} {
		req := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/hover","params":{"textDocument":{"uri":"$URI/proj/a/a.canon"},"position":{"line":%d,"character":4}}}`, id, line)
		must(t, s.sendLine([]string{req}))
	}
	cases := map[int]string{1: "[" + strings.Join(ints, ", ") + "]", 2: "[" + strings.Join(words, ", ") + "]"}
	for id, full := range cases {
		shown := s.hoverValue(t, id)
		text, ok := strings.CutSuffix(shown, lineSep+cutMark)
		if !ok || len(text) > hoverBytes || len(text) < hoverBytes-utf8.UTFMax || !utf8.ValidString(text) || !strings.HasPrefix(full, text) {
			t.Errorf("hover %d shows %d bytes, cut %v; want at most %d bytes of the text, on a character, then %q", id, len(text), ok, hoverBytes, cutMark)
		}
	}
	must(t, s.in.Close())
	<-done
}

// hoverValue is the fenced value of the hover answering request id.
func (s *session) hoverValue(t *testing.T, id int) string {
	t.Helper()
	for _, m := range s.rec.since(0) {
		var r struct {
			ID     *int         `json:"id"`
			Result *hoverResult `json:"result"`
		}
		if json.Unmarshal(m, &r) != nil || r.ID == nil || *r.ID != id || r.Result == nil {
			continue
		}
		_, value, ok := strings.Cut(r.Result.Contents.Value, valueLabel+fenceOpen)
		if ok {
			return strings.TrimSuffix(value, fenceClose)
		}
	}
	t.Fatalf("no hover value for request %d", id)
	return ""
}
