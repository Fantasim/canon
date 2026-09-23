package comments

// Blocky has two in-body comment blocks over the limit, sharing one ratchet key.
func Blocky() {
	// This is a four line comment block sitting inside the function body, well past
	// the three line limit comment-block enforces, so it should be flagged as a
	// finding here.
	// Fourth line of the same block.
	_ = 1

	// A second over-limit block further down in the same function: four lines here
	// too, so the ratchet key (rule, file, symbol) repeats and the count grows
	// instead of a new baseline row appearing for it.
	// Fourth line again.
	_ = 2
}
