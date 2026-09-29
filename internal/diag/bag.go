package diag

import (
	"bytes"
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
	kept, _ := b.view()
	return kept
}

// Summary counts the bag's findings, the dropped ones included, as one package (API.md §4.3).
func (b *Bag) Summary() Summary {
	kept, dropped := b.view()
	s := Summary{Packages: 1}
	s.Errors, s.Warnings = count(kept)
	s.Errors += dropped.Errors
	s.Warnings += dropped.Warnings
	if dropped.Errors+dropped.Warnings > 0 {
		s.Truncated = []Truncation{dropped}
	}
	return s
}

func (b *Bag) add(f Finding) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.findings = append(b.findings, f)
}

// view sorts a copy of the findings in a total order, keeps the least of each run of
// duplicates, then splits the result at the bag's limit: the same set of reports gives the
// same view whatever order, or goroutines, they came from.
func (b *Bag) view() ([]Finding, Truncation) {
	b.mu.Lock()
	all := slices.Clone(b.findings)
	limit := b.max
	b.mu.Unlock()
	keyed := make([]keyedFinding, len(all))
	for i, f := range all {
		keyed[i] = keyedFinding{f: f, l: locate(b.files, f)}
	}
	// Two findings in files b.files does not know (Path "") at equal offsets tie: the Span.File
	// kept varies with report order, and no output reads it.
	slices.SortFunc(keyed, b.compareKeyed)
	var out []Finding
	for i := range keyed {
		if i == 0 || !duplicate(&keyed[i-1].l, &keyed[i].l) {
			out = append(out, keyed[i].f)
		}
	}
	cut := min(limit, len(out))
	dropped := Truncation{Package: b.pkg}
	dropped.Errors, dropped.Warnings = count(out[cut:])
	return out[:cut:cut], dropped
}

func count(fs []Finding) (errs, warnings int) {
	for _, f := range fs {
		switch f.Severity {
		case Error:
			errs++
		case Warning:
			warnings++
		case Runtime:
		}
	}
	return errs, warnings
}

// keyedFinding is a finding with its resolved form, which its order reads.
type keyedFinding struct {
	f Finding
	l Located
}

// compareKeyed is compareLocated, then the finding's own offsets, then its file's content,
// read only on a tie: two findings that resolve alike still sort one way, and never by FileID,
// which a persistent file set gives an edited file anew.
func (b *Bag) compareKeyed(x, y keyedFinding) int {
	c := cmp.Or(
		compareLocated(&x.l, &y.l),
		cmp.Compare(x.f.Span.Start, y.f.Span.Start),
		cmp.Compare(x.f.Span.End, y.f.Span.End),
	)
	if c != 0 {
		return c
	}
	return bytes.Compare(b.files.Content(x.f.Span.File), b.files.Content(y.f.Span.File))
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
