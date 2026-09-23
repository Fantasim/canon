package pkg

// F does not type-check.
func F() int {
	return "x" + 42
}
