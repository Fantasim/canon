package must_test

import (
	"errors"
	"runtime"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/must"
)

// recorder is a testing.TB that records Fatal and ends its goroutine, as the real one does.
type recorder struct {
	testing.TB
	fatal   []any
	helpers int
}

func (r *recorder) Helper() { r.helpers++ }

func (r *recorder) Fatal(args ...any) {
	r.fatal = append(r.fatal, args...)
	runtime.Goexit()
}

var errBoom = errors.New("boom")

// TestNoError proves that a nil error passes and an error calls Fatal (ADR-0023 section 9.6).
func TestNoError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantFatal bool
	}{
		{name: "nil passes", err: nil, wantFatal: false},
		{name: "error fails", err: errBoom, wantFatal: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			done := make(chan struct{})
			go func() {
				defer close(done)
				must.NoError(r, c.err)
			}()
			<-done
			if got := len(r.fatal) > 0; got != c.wantFatal {
				t.Fatalf("Fatal called = %v, want %v", got, c.wantFatal)
			}
			if c.wantFatal && !errors.Is(r.fatal[0].(error), c.err) {
				t.Fatalf("Fatal got %v, want %v", r.fatal, c.err)
			}
			if r.helpers != 1 {
				t.Fatalf("Helper calls = %d, want 1", r.helpers)
			}
		})
	}
}
