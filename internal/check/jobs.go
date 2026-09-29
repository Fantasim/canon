package check

// drainPkgJobs runs and removes every job of *jobs owned by p; running one may queue another (TYPES.md §13.2).
func drainPkgJobs[J any](jobs *[]J, p *pkgState, owner func(J) *pkgState, run func(J)) {
	for {
		var mine, rest []J
		for _, j := range *jobs {
			if owner(j) == p {
				mine = append(mine, j)
			} else {
				rest = append(rest, j)
			}
		}
		*jobs = rest
		if len(mine) == 0 {
			return
		}
		for _, j := range mine {
			run(j)
		}
	}
}
