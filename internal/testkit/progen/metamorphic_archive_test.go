package progen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// metaArchive is a failing case kept self-contained (decision log "A6 progen suites 3–4 —
// review calls"): the untouched files it needs, which the replay builds as its baseline, and the
// transformed file under variantDir; its want line names the transform.
func metaArchive(mc metaCase, base, variant *progen.Project, v verdict) *progen.Counterexample {
	files := base.Clone()
	src, _ := variant.Get(mc.tg.path)
	files.Set(variantDir+mc.tg.path, src)
	return &progen.Counterexample{
		Suite: suiteMeta, Name: mc.op.name, Case: mc.k, Seed: mc.seed, Sig: v.Sig, Packages: []string{mc.tg.pkg},
		Want: "no finding or output changes: " + mc.site.desc + " in " + mc.tg.path, Note: v.Text, Files: files,
	}
}

// trimMeta drops the files of p that neither the package's baseline nor the failure needs: the
// baseline stays what it was, and the transform still fails with sig. No file's text is cut.
func trimMeta(p *progen.Project, tg target, variant *progen.Project, sig string) *progen.Project {
	base := runMetaBuild(p, tg.pkg)
	src, _ := variant.Get(tg.path)
	pinned := func(name string) bool { return name == projectFile || name == tg.path }
	return trimFiles(p, pinned, func(q *progen.Project) bool {
		heartbeat()
		v := q.Clone()
		v.Set(tg.path, src)
		qb := runMetaBuild(q, tg.pkg)
		return compareMeta(base, qb).Kind == "" && sigKey(compareMeta(qb, runMetaBuild(v, tg.pkg)).Sig) == sigKey(sig)
	}, shrinkTries)
}

// trimFiles drops the files of p but the pinned ones while still holds (ddmin over files).
func trimFiles(p *progen.Project, pinned func(string) bool, still func(*progen.Project) bool, tries int) *progen.Project {
	names := p.Names()
	pins := make([]bool, len(names))
	for i, n := range names {
		pins[i] = pinned(n)
	}
	return subset(p, names, progen.Shrink(len(names), pins, func(keep []bool) bool {
		return still(subset(p, names, keep))
	}, tries))
}

// replayMeta re-runs a kept metamorphic archive from its files alone: its program is the
// baseline, the same program with its variantDir files in place the case.
func replayMeta(c *progen.Counterexample) verdict {
	if len(c.Packages) != 1 {
		return verdict{Kind: kindMalformed, Sig: kindMalformed, Text: "a metamorphic archive checks one package"}
	}
	prog := programOf(c.Files)
	variant := prog.Clone()
	for _, name := range c.Files.Names() {
		if rel, ok := strings.CutPrefix(name, variantDir); ok {
			src, _ := c.Files.Get(name)
			variant.Set(rel, src)
		}
	}
	return compareMeta(runMetaBuild(prog, c.Packages[0]), runMetaBuild(variant, c.Packages[0]))
}

// metaCrashCase is case k of the metamorphic suite rebuilt whole, for keepCrash.
func metaCrashCase(c *corpus, k int, seed uint64) (*progen.Counterexample, target, bool) {
	ops := metaOperators()
	mc, ok := drawMeta(c, ops[k%len(ops)], k, seed)
	if !ok {
		return nil, target{}, false
	}
	variant, err := mc.variant(c.project)
	if err != nil {
		return nil, target{}, false
	}
	return metaArchive(mc, c.project, variant, verdict{}), mc.tg, true
}

// programOf is an archive's project without its harness files (harnessDir), which the compiler
// never reads.
func programOf(p *progen.Project) *progen.Project {
	q := p.Clone()
	for _, name := range p.Names() {
		if strings.HasPrefix(name, harnessDir) {
			q.Remove(name)
		}
	}
	return q
}
