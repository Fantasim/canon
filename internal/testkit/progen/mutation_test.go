package progen_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// operator is one rule-targeted mutation (DECISIONS 200): applied at a site of an example, it
// must make the compiler report code, at the site, and nothing else but the also codes the
// spec expects there too.
type operator struct {
	code  diag.Code
	rule  string // the owning document's rule the operator targets
	build bool   // a build meets the code, not a check
	many  bool   // the code may be reported more than once in the region, once per use
	loose bool   // the owning rule fixes no location: the finding may be anywhere
	also  []diag.Code
	sites func(tg target) []progen.Site
}

func op(code diag.Code, rule string, sites func(tg target) []progen.Site) operator {
	return operator{code: code, rule: rule, sites: sites}
}

// name is how counterexamples name an operator: its code and rule.
func (o operator) name() string { return string(o.code) + " " + o.rule }

// run is what one case checks: the selected packages and the active layers.
type run struct {
	pkgs   []string
	layers []string
}

// placed is a site with the run whose check must report its finding.
type placed struct {
	run
	site progen.Site
}

// placements are every site of o in the corpus.
func (o operator) placements(c *corpus) []placed {
	var out []placed
	for _, tg := range c.targets {
		for _, s := range o.sites(tg) {
			if s.Path == "" {
				s.Path = tg.path
			}
			r := run{pkgs: []string{tg.pkg}, layers: s.Layers}
			if len(s.Packages) > 0 {
				r.pkgs = s.Packages
			}
			out = append(out, placed{run: r, site: s})
		}
	}
	return out
}

// TestMutations applies every operator of the catalogue to the examples, at one seeded site in
// the short run, at -progen.n seeded sites round the catalogue in the nightly one (DECISIONS
// 200). Each case must report its operator's code at its site and nothing else.
func TestMutations(t *testing.T) {
	ops := catalogue()
	if supervise(t, testMutations, total(len(ops))) {
		return
	}
	c := examples(t)
	from, to := cases(len(ops))
	for i := from; i < to; i++ {
		o := ops[i%len(ops)]
		t.Run(fmt.Sprintf("%s_%d", o.code, i), func(t *testing.T) {
			announce(suiteMutation, o.name(), i, caseSeed(suiteMutation, i))
			runOperator(t, c, o, i)
		})
	}
}

func runOperator(t *testing.T, c *corpus, o operator, k int) {
	t.Helper()
	seed := caseSeed(suiteMutation, k)
	sites := o.placements(c)
	if len(sites) == 0 {
		fail(t, "%s (%s): no site in the examples", o.code, o.rule)
		return
	}
	p := progen.Pick(progen.NewRand(seed), sites)
	m, err := p.site.Mutate(c.project)
	if err != nil {
		fail(t, "%s seed %d: %v", o.name(), seed, err)
		return
	}
	v := judge(o, p.run, m.At, m.Project)
	if v.Kind == "" || reported(t, suiteMutation, o.name(), seed, v) {
		return
	}
	report(t, shrinkMutation(failure{o: o, run: p.run, m: m, v: v, k: k, seed: seed}), v)
}

// verdict is how a case failed its property: Kind is the kind of failure ("" for none), Sig its
// signature, free of the names and places a site or shrinking changes, Text the whole story,
// Detail a stack if any.
type verdict struct {
	Kind, Sig, Text, Detail string
}

// judge runs the mutated project and compares its findings with the one o expects at the site.
func judge(o operator, r run, at progen.Place, p *progen.Project) verdict {
	return sorted(o, r, at, p).verdict(o.code, at.Region)
}

// judged is a mutated project's outcome, its findings sorted: hit, the operator's code at the
// site; other, every finding the operator does not expect.
type judged struct {
	out        progen.Outcome
	hit, other []progen.Finding
}

// sorted runs the mutated project and sorts its findings against the one o expects at the site.
func sorted(o operator, r run, at progen.Place, p *progen.Project) judged {
	j := judged{out: progen.Run(context.Background(), p, progen.RunOptions{
		Packages: r.pkgs, Roots: exampleRoots(), Layers: r.layers, Build: o.build, Targets: goAndJSON,
	})}
	for _, f := range j.out.Findings {
		inside := o.loose || f.Path == at.Path && at.Start <= f.Start && f.Start <= at.End
		switch {
		case f.Code == o.code && inside:
			j.hit = append(j.hit, f)
		case !inside || !slices.Contains(o.also, f.Code):
			j.other = append(j.other, f)
		}
	}
	if o.many && len(j.hit) > 1 {
		j.hit = j.hit[:1]
	}
	return j
}

// verdict is the case's verdict: "" when it reports code at the site and nothing else.
func (j judged) verdict(code diag.Code, at progen.Region) verdict {
	if v, bad := broken(j.out); bad {
		return v
	}
	return classify(code, j.hit, j.other, j.out.Err, at)
}

// broken is the verdict of a run that panicked or met an internal error: never acceptable,
// whatever it reported.
func broken(out progen.Outcome) (verdict, bool) {
	if out.Panic != "" {
		first, _, _ := strings.Cut(out.Panic, "\n")
		sig := kindPanic + " " + unplaced(first) + " in " + compilerFrames(out.Panic)
		return verdict{Kind: kindPanic, Sig: sig, Text: "panic " + first, Detail: out.Panic}, true
	}
	if errors.Is(out.Err, build.ErrInternal) {
		return verdict{Kind: kindInternal, Sig: kindInternal + " " + unplaced(out.Err.Error()), Text: "internal " + out.Err.Error()}, true
	}
	return verdict{}, false
}

func classify(code diag.Code, hit, other []progen.Finding, err error, at progen.Region) verdict {
	sig := shapes(other)
	switch {
	case len(hit) == 0 && slices.ContainsFunc(other, func(f progen.Finding) bool { return f.Code == code }):
		return verdict{Kind: kindMisplaced, Sig: kindMisplaced + " " + sig, Text: fmt.Sprintf("misplaced %s, want bytes %d..%d", describe(other), at.Start, at.End)}
	case len(hit) == 0 && err != nil && len(other) == 0:
		return verdict{Kind: kindError, Sig: kindError + " " + unplaced(err.Error()), Text: fmt.Sprintf("error %v", err)}
	case len(hit) == 0:
		return verdict{Kind: kindMissing, Sig: kindMissing + " " + sig, Text: fmt.Sprintf("missing %s, got %s", code, describe(other))}
	case len(other) > 0:
		return verdict{Kind: kindExtra, Sig: kindExtra + " " + sig, Text: fmt.Sprintf("extra %s besides %s", describe(other), describe(hit))}
	case len(hit) > 1:
		return verdict{Kind: kindRepeated, Sig: kindRepeated + " " + shapes(hit), Text: fmt.Sprintf("repeated %s", describe(hit))}
	}
	return verdict{}
}

// describe lists findings with their messages.
func describe(fs []progen.Finding) string {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, fmt.Sprintf("%v %q", f, f.Message))
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

// failure is a mutation case that failed: its operator, run, mutated project, verdict, number
// and seed.
type failure struct {
	o    operator
	run  run
	m    progen.Mutated
	v    verdict
	k    int
	seed uint64
}

// shrinkMutation shrinks a failing mutation, keeping every region and file the site wrote, the
// verdict's signature, and no finding the unshrunk case did not report: a leftover of shrinking
// is never born with the archive. A case born missing or broken keeps its site's precondition.
func shrinkMutation(f failure) *progen.Counterexample {
	pins := append([]progen.Place{f.m.At}, f.m.Written...)
	base := reportedBy(sorted(f.o, f.run, f.m.At, f.m.Project).out.Findings)
	placed := f.v.Kind == kindMissing || slices.Contains(crashKinds, f.v.Kind)
	small, pins := shrinkProject(f.m.Project, pins, func(q *progen.Project, pins []progen.Place) bool {
		heartbeat()
		j := sorted(f.o, f.run, pins[0], q)
		if j.verdict(f.o.code, pins[0].Region).Sig != f.v.Sig || !base.covers(j.other) {
			return false
		}
		return !placed || stillPlaced(f, q, pins[1:])
	}, shrinkTries)
	at := pins[0]
	v := judge(f.o, f.run, at, small)
	return &progen.Counterexample{
		Suite: suiteMutation, Name: f.o.name(), Case: f.k, Seed: f.seed, Sig: v.Sig,
		Packages: f.run.pkgs, Layers: f.run.layers, Want: wantOf(f.o, at), Note: v.Text, Files: small,
	}
}

// wantOf is the want header of a mutation case: the code, where it is expected, the mode.
func wantOf(o operator, at progen.Place) string {
	mode := modeCheck
	if o.build {
		mode = modeBuild
	}
	return fmt.Sprintf("%s %s %d %d %s", o.code, at.Path, at.Start, at.End, mode)
}

// shrinkProject drops the files a failure does not need, then lines and tokens of every kept
// file, never project.canon, a pinned file or a pinned region; still sees where the pins lie.
func shrinkProject(p *progen.Project, pins []progen.Place, still func(*progen.Project, []progen.Place) bool, tries int) (*progen.Project, []progen.Place) {
	names := p.Names()
	pinned := make([]bool, len(names))
	for i, n := range names {
		pinned[i] = n == projectFile || slices.ContainsFunc(pins, func(pl progen.Place) bool { return pl.Path == n })
	}
	small := subset(p, names, progen.Shrink(len(names), pinned, func(keep []bool) bool {
		return still(subset(p, names, keep), pins)
	}, tries))
	for _, name := range small.Names() {
		pins = shrinkFile(small, name, pins, still, tries)
	}
	return small, pins
}

// shrinkFile shrinks one file of p in place, keeping its pins, and returns where all pins lie.
func shrinkFile(p *progen.Project, name string, pins []progen.Place, still func(*progen.Project, []progen.Place) bool, tries int) []progen.Place {
	var mine []int
	var regions []progen.Region
	for i, pl := range pins {
		if pl.Path == name {
			mine, regions = append(mine, i), append(regions, pl.Region)
		}
	}
	moved := func(rs []progen.Region) []progen.Place {
		out := slices.Clone(pins)
		for k, i := range mine {
			out[i].Region = rs[k]
		}
		return out
	}
	src, _ := p.Get(name)
	src, regions = progen.ShrinkText(src, regions, func(text []byte, rs []progen.Region) bool {
		q := p.Clone()
		q.Set(name, text)
		return still(q, moved(rs))
	}, tries)
	p.Set(name, src)
	return moved(regions)
}

func subset(p *progen.Project, names []string, keep []bool) *progen.Project {
	q := p.Clone()
	for i, n := range names {
		if !keep[i] {
			q.Remove(n)
		}
	}
	return q
}

// replayMutation re-runs a kept mutation counterexample with its catalogue operator; Kind ""
// when the bug it was born with is gone, or, with no born line yet, when it passes.
func replayMutation(c *progen.Counterexample) verdict {
	f := strings.Fields(c.Want)
	if len(f) != wantFields {
		return verdict{Kind: kindMalformed, Text: "malformed want " + c.Want}
	}
	start, err1 := strconv.Atoi(f[2])
	end, err2 := strconv.Atoi(f[3])
	if err1 != nil || err2 != nil {
		return verdict{Kind: kindMalformed, Text: "malformed want " + c.Want}
	}
	o := operator{code: diag.Code(f[0]), build: f[4] == modeBuild}
	if i := slices.IndexFunc(catalogue(), func(op operator) bool { return op.name() == c.Name }); i >= 0 {
		o = catalogue()[i]
	}
	at := progen.Place{Path: f[1], Region: progen.Region{Start: start, End: end}}
	j := sorted(o, run{pkgs: c.Packages, layers: c.Layers}, at, c.Files)
	if c.Born == "" {
		return j.verdict(o.code, at.Region)
	}
	v := j.bornVerdict(o.code, at.Region, c.Born, c.Guard)
	if v.Kind == "" && c.Open == "" {
		return withinLeft(v, c.Left)
	}
	return v
}
