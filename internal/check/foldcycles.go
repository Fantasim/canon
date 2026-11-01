package check

import "github.com/fantasim/canonlang/internal/diag"

// cycle closes the path b.stack[at:] with h: E4301 once for its consts, broken; a cycle through defaults alone is no constant (E3015).
func (b *builds) cycle(at int, h hop) {
	hops := append(append([]hop(nil), b.stack[at:]...), h)
	var consts []int // the indexes of the consts among hops[:len(hops)-1], the path's own hops
	for i, s := range hops[:len(hops)-1] {
		if s.obj.kind == ObjConst {
			consts = append(consts, i)
		}
	}
	if len(consts) == 0 {
		b.unready = true
		return
	}
	b.broken = true
	first := consts[0]
	for _, i := range consts {
		if compareFileSpans(declPlace(hops[i].obj), declPlace(hops[first].obj)) < 0 {
			first = i
		}
	}
	if first == 0 {
		first = len(hops) - 1 // hops[0] is read again by h, the use closing the cycle
	}
	b.c.reportConstCycle(hops, first, consts)
}

// reportConstCycle is E4301 at the read of the cycle's first const, the path the const before it, as eval's (EVALUATION.md §3.2; DECISIONS 187).
func (c *checker) reportConstCycle(hops []hop, first int, consts []int) {
	n := len(hops) - 1
	start := first % n
	var names []string
	closing := hops[consts[len(consts)-1]].obj
	for i := range n {
		if s := hops[(start+i)%n]; s.obj.kind == ObjConst {
			names = append(names, s.obj.name)
		}
	}
	for i := 1; i <= n; i++ {
		if s := hops[(start+n-i)%n]; s.obj.kind == ObjConst {
			closing = s.obj
			break
		}
	}
	names = append(names, names[0])
	broken := false
	for _, i := range consts {
		broken = broken || c.info.Broken[hops[i].obj]
	}
	if at := hops[first]; !broken && at.file != nil {
		c.report(c.declEnv(hops[start].obj), diag.E4301.At(at.file.Span(at.use), names).Path(closing.name))
	}
	for _, i := range consts {
		c.breakObj(hops[i].obj)
	}
}
