package comments

// ADRNote cites a decision across two lines, which is narration, not a bare reference.
// See ADR-0012 for the background on why this function exists in its current form.
func ADRNote() {}

// HistoryNote used to return an error here, but the behavior was simplified later on.
func HistoryNote() {}

func TODONote() {
	// TODO: replace this with the real implementation once the API stabilizes.
	_ = 1
}
