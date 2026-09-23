package ratchet

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

type Entry struct {
	Rule   string
	File   string
	Symbol string
	Detail string
	Count  int
	Value  int
}

func (e Entry) key() string {
	return finding.Finding{Rule: e.Rule, File: e.File, Symbol: e.Symbol, Detail: e.Detail}.Key()
}

type Baseline map[string]Entry

// Load reads a baseline; a missing file is an empty baseline and exists=false.
func Load(path string) (b Baseline, exists bool, err error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return Baseline{}, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Parse(f), true, nil
}

// Parse reads baseline text; malformed rows are skipped.
func Parse(r io.Reader) Baseline {
	b := Baseline{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, scanBuf), scanBuf)
	for sc.Scan() {
		l := sc.Text()
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		c := strings.Split(l, "\t")
		if len(c) != baselineCols {
			continue
		}
		n, _ := strconv.Atoi(c[colCount])
		v, _ := strconv.Atoi(c[colValue])
		e := Entry{Rule: c[colRule], File: c[colFile], Symbol: c[colSymbol], Detail: c[colDetail], Count: n, Value: v}
		b[e.key()] = e
	}
	return b
}

// Build turns findings into a baseline: one entry per key, count and max value.
func Build(fs []finding.Finding) Baseline {
	b := Baseline{}
	for _, f := range fs {
		k := f.Key()
		e, ok := b[k]
		if !ok {
			e = Entry{Rule: f.Rule, File: f.File, Symbol: f.Symbol, Detail: f.Detail}
		}
		e.Count++
		e.Value = max(e.Value, f.Value)
		b[k] = e
	}
	return b
}

func (b Baseline) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	keys := slices.Sorted(maps.Keys(b))
	var sb strings.Builder
	sb.WriteString(header)
	for _, k := range keys {
		e := b[k]
		fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\t%d\t%d\n", e.Rule, e.File, e.Symbol, finding.Clean(e.Detail), e.Count, e.Value)
	}
	if err := os.WriteFile(path, []byte(sb.String()), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Tighten lowers every in-scope entry to what the tree has now and drops the ones at
// zero. It never raises or adds an entry.
func Tighten(b Baseline, cur []finding.Finding, inScope func(string) bool) (Baseline, int) {
	now := Build(cur)
	out := Baseline{}
	dropped := 0
	for k, e := range b {
		if !inScope(e.File) {
			out[k] = e
			continue
		}
		n, ok := now[k]
		if !ok {
			dropped++
			continue
		}
		if n.Count < e.Count || n.Value < e.Value {
			dropped++
		}
		e.Count = min(e.Count, n.Count)
		e.Value = min(e.Value, n.Value)
		out[k] = e
	}
	return out, dropped
}

// Grown is a finding whose measured value went up against its baseline entry.
type Grown struct {
	F   finding.Finding
	Was int
}

type Verdict struct {
	New      []finding.Finding
	Grew     []Grown
	Enforced []finding.Finding
	Fixed    int
}

func (v Verdict) Failed() bool { return len(v.New)+len(v.Grew)+len(v.Enforced) > 0 }

// Compare judges the current findings against the baseline. mode gives each rule's
// effective mode; inScope restricts the judgement to the files --changed looks at.
func Compare(cur []finding.Finding, b Baseline, mode func(string) rules.Mode, inScope func(string) bool) Verdict {
	var v Verdict
	groups := map[string][]finding.Finding{}
	var order []string
	for _, f := range cur {
		if !inScope(f.File) {
			continue
		}
		k := f.Key()
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], f)
	}
	for _, k := range order {
		g := groups[k]
		switch mode(g[0].Rule) {
		case rules.Enforce:
			v.Enforced = append(v.Enforced, g...)
		case rules.Ratchet:
			v.judge(g, b[k])
		case rules.Observe, rules.Off:
			// neither gates the check
		}
	}
	for k, e := range b {
		if g, ok := groups[k]; inScope(e.File) && (!ok || len(g) < e.Count) {
			v.Fixed += e.Count - len(g)
		}
	}
	return v
}

func (v *Verdict) judge(g []finding.Finding, e Entry) {
	sort.SliceStable(g, func(i, j int) bool { return g[i].Line < g[j].Line })
	if len(g) > e.Count {
		v.New = append(v.New, g[e.Count:]...)
		return
	}
	top := g[0]
	for _, f := range g {
		if f.Value > top.Value {
			top = f
		}
	}
	if e.Count > 0 && top.Value > e.Value {
		v.Grew = append(v.Grew, Grown{F: top, Was: e.Value})
	}
}
