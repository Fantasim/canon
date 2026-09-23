package api

import "testing"

func TestHelper(t *testing.T) {
	TestOnly()
	go worker()
	_ = "only-here" + "only-here"
	_ = 42
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
