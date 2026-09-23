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

// view sorts and deduplicates a copy of the findings, then splits it at the bag's limit.
func (b *Bag) view() ([]Finding, Truncation) {
	b.mu.Lock()
	all := slices.Clone(b.findings)
	limit := b.max
	b.mu.Unlock()
	keyed := sortFindings(b.files, all)
	unique := keyed[:0]
	for i, k := range keyed {
		if i == 0 || !sameFinding(keyed[i-1], k) {
			unique = append(unique, k)
		}
	}
	out := make([]Finding, len(unique))
	for i, k := range unique {
		out[i] = k.f
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

// keyedFinding is a finding with the position its order reads (API.md F2).
type keyedFinding struct {
	f         Finding
	path      string
	line, col int
}

// sortFindings orders findings stably by file, line, column, code and message (API.md F2).
func sortFindings(files Files, fs []Finding) []keyedFinding {
	keyed := make([]keyedFinding, len(fs))
	for i, f := range fs {
		k := keyedFinding{f: f, path: files.Path(f.Span.File)}
		if k.path != "" {
			k.line, k.col = files.Position(f.Span.File, f.Span.Start)
		}
		keyed[i] = k
	}
	slices.SortStableFunc(keyed, func(a, b keyedFinding) int {
		return cmp.Or(
			cmp.Compare(a.path, b.path),
			cmp.Compare(a.line, b.line),
			cmp.Compare(a.col, b.col),
			cmp.Compare(a.f.Code, b.f.Code),
			cmp.Compare(a.f.Message, b.f.Message),
		)
	})
	return keyed
}

// sameFinding tells a duplicate, of which the first produced is kept (EVALUATION.md §14).
func sameFinding(a, b keyedFinding) bool {
	return a.f.Severity == b.f.Severity && a.f.Code == b.f.Code && a.path == b.path &&
		a.line == b.line && a.col == b.col && a.f.Message == b.f.Message
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
