package diag_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// countingFiles counts the positions a bag resolves: the work its view does per finding.
type countingFiles struct {
	diag.Files
	positions int
}

func (c *countingFiles) Position(id source.FileID, p source.Pos) (line, col int) {
	c.positions++
	return c.Files.Position(id, p)
}

// reportAsking reports n findings, each at a line of its own, and asks the summary after every
// one, as a checker does around each step of its pass; then it reads the findings three times.
func reportAsking(n int) (positions, errors int) {
	files := &countingFiles{Files: diag.MemFiles{{Path: "p.canon", Content: strings.Repeat("x\n", n)}}}
	bag := diag.NewBag(files, "p")
	for i := range n {
		diag.E1004.At(source.Span{File: 1, Start: source.Pos(2 * i)}).Report(bag)
		errors = bag.Summary().Errors
	}
	for range 3 {
		bag.Findings()
	}
	return files.positions, errors
}

// API.md F7, NFR-01: four times the findings cost about four times the positions, not sixteen.
func TestBagWorkGrowsLinearly(t *testing.T) {
	small, smallErrors := reportAsking(200)
	large, largeErrors := reportAsking(800)
	if smallErrors != 200 || largeErrors != 800 {
		t.Fatalf("summary counts %d and %d errors", smallErrors, largeErrors)
	}
	if large > small*5 {
		t.Errorf("200 findings cost %d positions, 800 cost %d: more than 5 times", small, large)
	}
}

// reportCounting reports n findings into a bag that keeps limit of them, asks the error count
// after each, then reads the summary and the findings.
func reportCounting(n, limit int) (positions int, s diag.Summary) {
	files := &countingFiles{Files: diag.MemFiles{{Path: "p.canon", Content: strings.Repeat("x\n", n)}}}
	bag := diag.NewBag(files, "p")
	bag.Truncate(limit)
	for i := range n {
		diag.E1004.At(source.Span{File: 1, Start: source.Pos(2 * i)}).Report(bag)
		if bag.ErrorCount() != i+1 {
			return 0, diag.Summary{}
		}
	}
	bag.Findings()
	return files.positions, bag.Summary()
}

// API.md F7, NFR-01: past its limit a bag still counts every error, at a cost in proportion to
// the findings.
func TestBagCountsPastItsLimitLinearly(t *testing.T) {
	const limit = 100
	small, smallSum := reportCounting(400, limit)
	large, largeSum := reportCounting(1600, limit)
	if smallSum.Errors != 400 || largeSum.Errors != 1600 || largeSum.Truncated[0].Errors != 1600-limit {
		t.Fatalf("summaries %+v and %+v", smallSum, largeSum)
	}
	if large > small*5 {
		t.Errorf("400 findings cost %d positions, 1600 cost %d: more than 5 times", small, large)
	}
}

// API.md F7: a summary read while findings come in is one snapshot: past the limit it lists the
// truncation, never more errors than the bag keeps with none dropped.
func TestBagSummaryIsOneSnapshot(t *testing.T) {
	const limit, total = 5, 300
	files := diag.MemFiles{{Path: "p.canon", Content: strings.Repeat("x\n", total)}}
	bag := diag.NewBag(files, "p")
	bag.Truncate(limit)
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range total {
			diag.E1004.At(source.Span{File: 1, Start: source.Pos(2 * i)}).Report(bag)
		}
	})
	for range total {
		if s := bag.Summary(); s.Errors > limit && len(s.Truncated) == 0 {
			t.Errorf("%d errors, none dropped, with a limit of %d", s.Errors, limit)
		}
	}
	wg.Wait()
}

// API.md F7: reading a bag again, with nothing reported in between, resolves nothing again.
func TestBagViewIsKept(t *testing.T) {
	files := &countingFiles{Files: diag.MemFiles{{Path: "p.canon", Content: "x\ny\n"}}}
	bag := diag.NewBag(files, "p")
	diag.E1004.At(source.Span{File: 1}).Report(bag)
	diag.E1004.At(source.Span{File: 1, Start: 2}).Report(bag)
	bag.Findings()
	bag.Summary()
	before := files.positions
	for range 10 {
		bag.Findings()
		bag.Summary()
	}
	if files.positions != before {
		t.Errorf("%d positions after the first read, %d after ten more", before, files.positions)
	}
	diag.E1004.At(source.Span{File: 1, Start: 3}).Report(bag)
	if n := len(bag.Findings()); n != 3 {
		t.Errorf("a finding reported after a read gives %d findings", n)
	}
}
