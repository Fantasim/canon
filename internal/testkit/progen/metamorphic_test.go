package progen_test

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	shortMeta      = 40
	metaNightlyCap = 5000 // two in-process builds a case, no Go compiled: the 5 relations share it
	metaBlank      = "blank line"
	metaComment    = "comment line"
	metaSpacing    = "spacing"
	metaRename     = "rename"
	metaReorder    = "reorder"
	testMeta       = "TestMetamorphic"
	variantDir     = harnessDir + "variant/" // an archive's transformed file, by its project path
)

// metaOp is one relation of DECISIONS 200 item 4: each site it finds in a clean file of the
// examples changes no finding and no output of the file's package.
type metaOp struct {
	name, rule string
	sites      func(tg target) []metaSite
}

// metaSite is one application of a relation: its edits of the target file, and what it does.
type metaSite struct {
	edits []progen.Edit
	desc  string
}

// metaOperators are DECISIONS 200 item 4's relations; public names and order are outputs (CODEGEN.md §2.7, §3).
func metaOperators() []metaOp {
	return []metaOp{
		{name: metaBlank, rule: "GRAMMAR.md §3.1 (a run of line breaks is one NL), §9.1 (never after a doc block)", sites: blankSites},
		{name: metaComment, rule: "GRAMMAR.md §3.1 (comments are trivia), §9.1 (they keep a doc attached)", sites: commentSites},
		{name: metaSpacing, rule: "GRAMMAR.md §2.1 (whitespace is trivia), §3.1 (no line break added)", sites: spacingSites},
		{name: metaRename, rule: "CODEGEN.md §2.1 EMT-07 (a local declaration is never emitted)", sites: renameSites},
		{name: metaReorder, rule: "CODEGEN.md §2.7 (locals are not emitted), GRAMMAR.md §7.1 (project keys in any order)", sites: reorderSites},
	}
}

// TestMetamorphic is DECISIONS 200 item 4: a seeded site of each relation, round the relations,
// drawn among the files where it applies, changes no finding and no output; each child logs how
// many cases each relation ran.
func TestMetamorphic(t *testing.T) {
	n := min(total(shortMeta), metaNightlyCap)
	if supervise(t, testMeta, n) {
		return
	}
	c := examples(t)
	ops := metaOperators()
	ran := map[string]int{}
	for i := *flagFrom; i < n; i++ {
		seed := caseSeed(suiteMeta, i)
		op := ops[i%len(ops)]
		announce(suiteMeta, op.name, i, seed)
		t.Run(fmt.Sprintf("%s_%d", slug(op.name), i), func(t *testing.T) {
			if runMeta(t, c, op, i, seed) {
				ran[op.name]++
			}
		})
	}
	counts := make([]string, 0, len(ops))
	for _, op := range ops {
		counts = append(counts, fmt.Sprintf("%s %d", op.name, ran[op.name]))
	}
	relay(t, "metamorphic cases per relation: %s", strings.Join(counts, shapeSep))
}

// metaCase is one drawn case: its relation, number, seed, target file and site.
type metaCase struct {
	op   metaOp
	k    int
	seed uint64
	tg   target
	site metaSite
}

// runMeta runs case k of op; false when op applies to no file (a failure: a relation must run).
func runMeta(t *testing.T, c *corpus, op metaOp, k int, seed uint64) bool {
	t.Helper()
	mc, ok := drawMeta(c, op, k, seed)
	if !ok {
		fail(t, "%s: no file of the examples has a site (%s)", op.name, op.rule)
		return false
	}
	variant, err := mc.variant(c.project)
	if err != nil {
		fail(t, "%s seed %d: %v", op.name, seed, err)
		return false
	}
	v := compareMeta(baselineOutcome(c, mc.tg.pkg), runMetaBuild(variant, mc.tg.pkg))
	if v.Kind == "" || reported(t, suiteMeta, op.name, seed, v) {
		return true
	}
	report(t, metaArchive(mc, trimMeta(c.project, mc.tg, variant, v.Sig), variant, v), v)
	return true
}

// drawMeta is case k: a seeded file among those where op has a site, then a seeded site of it.
func drawMeta(c *corpus, op metaOp, k int, seed uint64) (metaCase, bool) {
	tgs := metaTargets(c, op)
	if len(tgs) == 0 {
		return metaCase{}, false
	}
	r := progen.NewRand(seed)
	tg := progen.Pick(r, tgs)
	return metaCase{op: op, k: k, seed: seed, tg: tg, site: progen.Pick(r, op.sites(tg))}, true
}

// variant is p with the case's site applied to its target file.
func (mc metaCase) variant(p *progen.Project) (*progen.Project, error) {
	m, err := progen.Site{Path: mc.tg.path, Edits: mc.site.edits}.Mutate(p)
	if err != nil {
		return nil, err
	}
	return m.Project, nil
}

var (
	metaMu       sync.Mutex
	metaApplies  = map[string][]target{}
	metaBaseline = map[string]progen.Outcome{}
)

// metaTargets are the corpus files where op has a site, computed once per relation.
func metaTargets(c *corpus, op metaOp) []target {
	metaMu.Lock()
	defer metaMu.Unlock()
	if tgs, ok := metaApplies[op.name]; ok {
		return tgs
	}
	var tgs []target
	for _, tg := range c.targets {
		if len(op.sites(tg)) > 0 {
			tgs = append(tgs, tg)
		}
	}
	metaApplies[op.name] = tgs
	return tgs
}

// baselineOutcome is pkg's build of the untouched corpus, computed once per package.
func baselineOutcome(c *corpus, pkg string) progen.Outcome {
	metaMu.Lock()
	defer metaMu.Unlock()
	if out, ok := metaBaseline[pkg]; ok {
		return out
	}
	out := runMetaBuild(c.project, pkg)
	metaBaseline[pkg] = out
	return out
}

// runMetaBuild builds pkg of p for Go and JSON in check mode (nothing written), the roots p
// shares with the examples redirected as theirs are (exampleRoots).
func runMetaBuild(p *progen.Project, pkg string) progen.Outcome {
	return progen.Run(context.Background(), p, progen.RunOptions{Packages: []string{pkg}, Roots: rootsFor(p), Build: true, Targets: goAndJSON})
}

// rootsFor is exampleRoots cut to the roots p's project file declares.
func rootsFor(p *progen.Project) map[string]string {
	src, _ := p.Get(projectFile)
	tg := target{path: projectFile, src: src, file: parse(projectFile, src)}
	declared := map[string]bool{}
	for _, b := range projectBlocks(tg, rootsKey) {
		declared[b.name] = true
	}
	roots := exampleRoots()
	maps.DeleteFunc(roots, func(name, _ string) bool { return !declared[name] })
	return roots
}

// compareMeta is "" when out reports what base does, message by message (places cut), and
// writes the same outputs; else signed by the findings gained and lost, or the output changed.
func compareMeta(base, out progen.Outcome) verdict {
	if v, bad := broken(out); bad {
		return v
	}
	if diff := findingDiff(base.Findings, out.Findings); diff != "" {
		return verdict{Kind: kindMetaFindings, Sig: kindMetaFindings + " " + diff, Text: fmt.Sprintf("findings changed: base %v, got %v", base.Findings, out.Findings)}
	}
	if d := diffOutputs(base.Outputs, out.Outputs); d != "" {
		return verdict{Kind: kindMetaOutput, Sig: kindMetaOutput + " " + d, Text: "output " + d}
	}
	return verdict{}
}

// findingDiff lists the shapes out reports more often than base ("+E2102") and less ("-W1002").
func findingDiff(base, out []progen.Finding) string {
	b, o := reportedBy(base), reportedBy(out)
	shape := func(key string) string {
		code, msg, _ := strings.Cut(key, " ")
		return shapeOf(diag.Code(code), msg)
	}
	var diff []string
	for _, key := range slices.Sorted(maps.Keys(o)) {
		if o[key] > b[key] {
			diff = append(diff, "+"+shape(key))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(b)) {
		if b[key] > o[key] {
			diff = append(diff, "-"+shape(key))
		}
	}
	return strings.Join(slices.Compact(diff), shapeSep)
}

// diffOutputs is "" for the same content at every path, else the first path that differs (DOCTRINE.md §5).
func diffOutputs(a, b []build.Output) string {
	ma, mb := outputMap(a), outputMap(b)
	paths := slices.Concat(slices.Collect(maps.Keys(ma)), slices.Collect(maps.Keys(mb)))
	slices.Sort(paths)
	for _, path := range slices.Compact(paths) {
		x, inA := ma[path]
		y, inB := mb[path]
		switch {
		case !inA:
			return "added " + path
		case !inB:
			return "dropped " + path
		case !bytes.Equal(x, y):
			return "changed " + path
		}
	}
	return ""
}
