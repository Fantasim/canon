package diag

import (
	"bytes"
	"cmp"
	"slices"
)

// view is a bag's findings as Findings reads them: sorted, deduplicated and cut at limit.
type view struct {
	n, limit int // the findings it was made from, and the bag's limit then
	kept     []Finding
	dropped  Truncation
}

// dupKey is what makes two findings one, and the first fields their order reads (EVALUATION.md §14).
type dupKey struct {
	path      string
	line, col int
	code      Code
	message   string
	severity  Severity
}

// sortKey is one finding to sort: its dupKey and its index in the findings it came from.
type sortKey struct {
	dupKey
	at int
}

// keyOf is the dupKey of f: its own span resolved, nothing else.
func keyOf(files Files, f *Finding) dupKey {
	l := locOf(files, f.Span)
	return dupKey{path: l.Path, line: l.Line, col: l.Col, code: f.Code, message: f.Message, severity: f.Severity}
}

func compareKeys(a, b *dupKey) int {
	return cmp.Or(
		cmp.Compare(a.path, b.path),
		cmp.Compare(a.line, b.line),
		cmp.Compare(a.col, b.col),
		cmp.Compare(a.code, b.code),
		cmp.Compare(a.message, b.message),
		cmp.Compare(a.severity, b.severity),
	)
}

// view is the bag's current view, made again only when findings came in or the limit moved
// since the last one: the same set of reports gives the same view whatever order, or
// goroutines, they came from.
func (b *Bag) view() view {
	b.mu.Lock()
	n, limit := len(b.findings), b.max
	if v := b.cached; v != nil && v.n == n && v.limit == limit {
		b.mu.Unlock()
		return *v
	}
	all := b.findings[:n:n] // findings are only appended: the first n never change
	b.mu.Unlock()
	v := b.makeView(all, limit)
	b.mu.Lock()
	b.cached = &v
	b.mu.Unlock()
	return v
}

// makeView sorts the findings in a total order by an index of their keys, so the findings
// themselves stay where they are, keeps the least of each run of duplicates, then splits the
// result at limit.
func (b *Bag) makeView(all []Finding, limit int) view {
	keys := make([]sortKey, len(all))
	for i := range all {
		keys[i] = sortKey{dupKey: keyOf(b.files, &all[i]), at: i}
	}
	// Two findings in files b.files does not know (Path "") at equal offsets tie: the Span.File
	// kept varies with report order, and no output reads it.
	slices.SortFunc(keys, func(x, y sortKey) int { return b.compareSort(all, x, y) })
	v := view{n: len(all), limit: limit, dropped: Truncation{Package: b.pkg}}
	var last *dupKey
	for i := range keys {
		if last != nil && *last == keys[i].dupKey {
			continue
		}
		last = &keys[i].dupKey
		if len(v.kept) < limit {
			v.kept = append(v.kept, all[keys[i].at])
			continue
		}
		v.dropped.add(keys[i].severity)
	}
	return v
}

func (t *Truncation) add(s Severity) {
	switch s {
	case Error:
		t.Errors++
	case Warning:
		t.Warnings++
	case Runtime:
	}
}

// compareSort is compareLocated, then the finding's own offsets, then its file's content, read
// only on a tie: two findings that resolve alike still sort one way, and never by FileID,
// which a persistent file set gives an edited file anew. The keys decide all but a tie.
func (b *Bag) compareSort(all []Finding, x, y sortKey) int {
	if c := compareKeys(&x.dupKey, &y.dupKey); c != 0 {
		return c
	}
	fx, fy := all[x.at], all[y.at]
	lx, ly := locate(b.files, fx), locate(b.files, fy)
	c := cmp.Or(
		compareLocated(&lx, &ly),
		cmp.Compare(fx.Span.Start, fy.Span.Start),
		cmp.Compare(fx.Span.End, fy.Span.End),
	)
	if c != 0 {
		return c
	}
	return bytes.Compare(b.files.Content(fx.Span.File), b.files.Content(fy.Span.File))
}
