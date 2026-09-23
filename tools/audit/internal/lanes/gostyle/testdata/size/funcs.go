package size

// LongLines has a body over the line limit.
func LongLines() {
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
}

// ManyStatements has more statements than the limit but a short-enough body.
func ManyStatements() {
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
	_ = 1
}

// SixParams has more parameters than the limit.
func SixParams(a, b, c, d, e, f int) int {
	return a + b + c + d + e + f
}

// FourResults has more results than the limit.
func FourResults() (int, int, int, int) {
	return 1, 1, 1, 1
}

// DeepNest nests if statements past the limit.
func DeepNest(x int) {
	if x > 0 {
		if x > 1 {
			if x > 2 {
				if x > 3 {
					_ = x
				}
			}
		}
	}
}

// NakedReturn has a bare return in a function longer than the limit.
func NakedReturn() (result int) {
	result = 1
	_ = result
	_ = result
	_ = result
	_ = result
	return
}
