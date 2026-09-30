package check

// UncompleteRecords sends every record of the session's checker back to a shell, so that the
// next expression checked against one completes it again, folding its bounds: a fold made while
// a swapped file is checked, which apply's guard exists for and no source can cause.
func UncompleteRecords(s *Session) {
	for _, o := range s.c.typeObjects { //canon:unordered every record is reset alike
		o.state = stateNone
	}
}
