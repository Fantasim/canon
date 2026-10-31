package must

import "testing"

// NoError fails tb at once, with err, when err is not nil.
func NoError(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}
