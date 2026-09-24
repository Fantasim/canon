package format

import (
	"slices"
	"unicode/utf8"
)

// measure is fits' state: the commands left to simulate, the width left on the line, and the
// printer's stack below them.
type measure struct {
	sim   []cmd
	w     int
	stack []cmd
	rest  int
}

// fitsTable simulates one command; it reports true when a line break ends the measure.
var fitsTable [kindCount]func(*measure, cmd) bool

func init() {
	fitsTable = [kindCount]func(*measure, cmd) bool{
		kText: func(m *measure, c cmd) bool { return m.take(c.d.text) }, kConcat: (*measure).concat,
		kLine:     func(m *measure, c cmd) bool { return c.mode == breakMode || m.take(space) },
		kSoftline: func(_ *measure, c cmd) bool { return c.mode == breakMode },
		kHardline: breaks, kComment: breaks, kBlank: breaks,
		kIfBreak: (*measure).ifBreak, kIndent: (*measure).inner, kGroup: (*measure).group,
		kFlat:   func(m *measure, c cmd) bool { return m.inner(cmd{c.ind, flatMode, c.d}) },
		kSuffix: func(m *measure, c cmd) bool { m.take(space + c.d.text); return true },
		kMultiline: func(m *measure, c cmd) bool {
			m.take(c.d.lines[0])
			return true
		},
		kRHS: (*measure).rhs,
	}
}

func breaks(*measure, cmd) bool { return true }

// fits simulates next flat, then the rest of the stack in its own modes, within w columns, up
// to the first line break.
func (p *printer) fits(next cmd, w int) bool {
	m := &measure{sim: []cmd{next}, w: w, stack: p.stack, rest: len(p.stack)}
	return m.run()
}

func (m *measure) run() bool {
	for m.w >= 0 {
		if len(m.sim) == 0 {
			if m.rest == 0 {
				return true
			}
			m.rest--
			m.sim = append(m.sim, m.stack[m.rest])
			continue
		}
		c := m.sim[len(m.sim)-1]
		m.sim = m.sim[:len(m.sim)-1]
		if fitsTable[c.d.kind](m, c) {
			return m.w >= 0
		}
	}
	return false
}

func (m *measure) take(s string) bool {
	m.w -= utf8.RuneCountInString(s)
	return false
}

func (m *measure) concat(c cmd) bool {
	for _, d := range slices.Backward(c.d.kids) {
		m.sim = append(m.sim, cmd{c.ind, c.mode, d})
	}
	return false
}

func (m *measure) inner(c cmd) bool {
	m.sim = append(m.sim, cmd{c.ind, c.mode, c.d.kids[0]})
	return false
}

func (m *measure) ifBreak(c cmd) bool {
	d := c.d.kids[1]
	if c.mode == breakMode {
		d = c.d.kids[0]
	}
	m.sim = append(m.sim, cmd{c.ind, c.mode, d})
	return false
}

// group is broken under a BREAK holder when it is hard, forced or single-line (DECISIONS 212).
func (m *measure) group(c cmd) bool {
	md := flatMode
	if c.mode == breakMode && (c.d.bit || c.d.hard || c.d.forced) {
		md = breakMode
	}
	return m.inner(cmd{c.ind, md, c.d})
}

// rhs follows the printer's rule A: broken, a value that fits flat on the next line ends the
// line after the operator.
func (m *measure) rhs(c cmd) bool {
	op, value := c.d.kids[0], c.d.kids[1]
	if c.d.flag {
		m.take(space)
	}
	next := c.ind + indentUnit
	if c.mode == breakMode && !c.d.alt && !value.hard && !value.forced {
		sub := &measure{sim: append(slices.Clone(m.sim), cmd{next, flatMode, value}), w: Width - next, stack: m.stack, rest: m.rest}
		if sub.run() {
			m.sim = append(m.sim, cmd{next, breakMode, hardlineDoc}, cmd{c.ind, c.mode, op})
			return false
		}
	}
	m.sim = append(m.sim, cmd{c.ind, c.mode, value}, cmd{c.ind, flatMode, spaceDoc}, cmd{c.ind, c.mode, op})
	return false
}
