package diag

import (
	"cmp"
	"slices"
	"sync"
)

// Bag collects the findings of one package, from any goroutine (IMPLEMENTATION-PLAN.md §4.4).
type Bag struct {
	files Files
	pkg   string

	mu       sync.Mutex
	max      int
	findings []Finding
	cached   *view // the view of the first cached.n findings under limit cached.limit, nil before the first

	tallyMu sync.Mutex
	tally   tally
}

// NewBag is an empty bag for package pkg that keeps DefaultMaxFindings findings.
func NewBag(files Files, pkg string) *Bag {
	return &Bag{files: files, pkg: pkg, max: DefaultMaxFindings}
}

// Truncate sets how many findings the bag keeps, the first ones in F2 order (API.md F7).
func (b *Bag) Truncate(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.max = max(n, 0)
}

// Findings is what the bag keeps: sorted, deduplicated and truncated (API.md F2, F7).
func (b *Bag) Findings() []Finding {
	return slices.Clone(b.view().kept)
}

// Summary counts the bag's findings, the dropped ones included, as one package (API.md §4.3).
func (b *Bag) Summary() Summary {
	if c := b.distinct(); c.holdsAll {
		return Summary{Errors: c.errors, Warnings: c.warnings, Packages: 1}
	}
	v := b.view()
	s := Summary{Packages: 1}
	s.Errors, s.Warnings = count(v.kept)
	s.Errors += v.dropped.Errors
	s.Warnings += v.dropped.Warnings
	if v.dropped.Errors+v.dropped.Warnings > 0 {
		s.Truncated = []Truncation{v.dropped}
	}
	return s
}

func (b *Bag) add(f Finding) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.findings = append(b.findings, f)
}

func count(fs []Finding) (errs, warnings int) {
	var t Truncation
	for _, f := range fs {
		t.add(f.Severity)
	}
	return t.Errors, t.Warnings
}

// Summary counts findings, dropped ones included (API.md §4.3).
type Summary struct {
	Errors, Warnings, Packages int
	Truncated                  []Truncation // in package-name order
}

// Truncation counts the findings a package dropped (API.md F7).
type Truncation struct {
	Package          string
	Errors, Warnings int
}

// Merge adds the counts of o, keeping Truncated in package-name order (API.md F7).
func (s Summary) Merge(o Summary) Summary {
	out := Summary{
		Errors:    s.Errors + o.Errors,
		Warnings:  s.Warnings + o.Warnings,
		Packages:  s.Packages + o.Packages,
		Truncated: slices.Concat(s.Truncated, o.Truncated),
	}
	slices.SortStableFunc(out.Truncated, func(a, b Truncation) int { return cmp.Compare(a.Package, b.Package) })
	return out
}

// notShown is the number of findings dropped over every package (API.md F15).
func (s Summary) notShown() int {
	n := 0
	for _, t := range s.Truncated {
		n += t.Errors + t.Warnings
	}
	return n
}
