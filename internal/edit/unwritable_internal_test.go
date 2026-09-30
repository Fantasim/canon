package edit

import (
	"errors"
	"testing"
)

// API.md N10: an unjournalable write is refused as a commit refuses it, an absolute display as reasonPlace.
func TestUnwritable(t *testing.T) {
	for _, c := range []struct {
		name     string
		w        work
		sentinel error
		reason   string
	}{
		{"visible", work{creates: []newFile{{display: "p/a.canon"}, {display: "@res/b.json"}}}, nil, ""},
		{"hidden", work{creates: []newFile{{display: "p/.h/a.canon"}}}, ErrUnwritable, reasonHidden},
		{"absolute", work{removes: []removal{{display: "/elsewhere/a.json"}}}, ErrChanges, reasonPlace},
		{"absolute before hidden", work{moves: []fileMove{{from: "/elsewhere/a.canon", to: "p/.a.canon"}}}, ErrChanges, reasonPlace},
	} {
		err := c.w.unwritable()
		var f *fault
		switch {
		case c.sentinel == nil && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.sentinel != nil && (!errors.Is(err, c.sentinel) || !errors.As(err, &f) || f.reason != c.reason):
			t.Errorf("%s: %v, want %v with reason %q", c.name, err, c.sentinel, c.reason)
		}
	}
}
