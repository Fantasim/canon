package workspace

// inputsSeen is how many times inputs changed so far.
func (s *snapFS) inputsSeen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ingen
}

// revised is the revision last computed on s while the files its loads read are the same: it is
// a function of them and of s's entries, each fixed once read (API.md S3).
func (s *snapFS) revised() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rev, s.rev != "" && s.revAt == s.ingen
}

// revise keeps rev, computed from the files s's loads had read at at, when they still are.
func (s *snapFS) revise(rev string, at int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at == s.ingen {
		s.rev, s.revAt = rev, at
	}
}
