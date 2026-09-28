package ir

import (
	"fmt"
	"regexp"
	resyntax "regexp/syntax"
	"slices"
)

// PatternOp is what one state of a PatternAutomaton does.
type PatternOp uint8

// PatternAt is the positions a PatternAnchor state asserts, as flags.
type PatternAt uint8

// PatternAutomaton is a pattern's Thompson automaton over code points, state 0 first: Go's own program without its captures and no-ops, so a search with one set of states per position accepts exactly what regexp.MatchString does (log 2026-09-25 "Pattern translator calls").
type PatternAutomaton struct{ States []PatternState }

// PatternState is one state; Out is the next state of every op but PatternAccept.
type PatternState struct {
	Op    PatternOp
	Out   int
	Alt   int       // PatternSplit's second branch
	At    PatternAt // PatternAnchor's positions
	Runes []rune    // PatternStep's code points as sorted lo, hi pairs; none never steps
}

// CompilePattern is re's automaton (EVALUATION.md §11.3, CODEGEN.md §5.12): the states of Go's program reachable from its start, so regexp's own size limits bound their count; case folding, multi-line anchors and word boundaries, which E1904 refuses, are ErrPattern.
func CompilePattern(re *regexp.Regexp) (PatternAutomaton, error) {
	tree, err := resyntax.Parse(re.String(), resyntax.Perl)
	if err != nil {
		return PatternAutomaton{}, fmt.Errorf(fmtPatternParse, ErrPattern, err)
	}
	prog, err := resyntax.Compile(tree.Simplify())
	if err != nil {
		return PatternAutomaton{}, fmt.Errorf(fmtPatternParse, ErrPattern, err)
	}
	b := automatonBuilder{prog: prog, index: map[int]int{}}
	b.state(prog.Start)
	var a PatternAutomaton
	for i := 0; i < len(b.order) && b.err == nil; i++ {
		inst := &prog.Inst[b.order[i]]
		a.States = append(a.States, b.convert(inst))
	}
	if b.err != nil {
		return PatternAutomaton{}, b.err
	}
	return a, nil
}

// automatonBuilder numbers Go's instructions as states in the order it meets them, breadth first.
type automatonBuilder struct {
	prog  *resyntax.Prog
	index map[int]int
	order []int
	err   error
}

// state is the state of instruction pc, past the no-ops and captures that lead from it.
func (b *automatonBuilder) state(pc int) int {
	for range b.prog.Inst {
		op := b.prog.Inst[pc].Op
		if op != resyntax.InstNop && op != resyntax.InstCapture {
			break
		}
		pc = int(b.prog.Inst[pc].Out)
	}
	if i, ok := b.index[pc]; ok {
		return i
	}
	b.index[pc] = len(b.order)
	b.order = append(b.order, pc)
	return b.index[pc]
}

// convert makes inst a state through instStates; an instruction without an entry is refused.
func (b *automatonBuilder) convert(inst *resyntax.Inst) PatternState {
	if int(inst.Op) < len(instStates) && instStates[inst.Op] != nil {
		return instStates[inst.Op](b, inst)
	}
	return b.refuse(inst)
}

func (b *automatonBuilder) refuse(inst *resyntax.Inst) PatternState {
	if b.err == nil {
		b.err = fmt.Errorf(fmtPatternInst, ErrPattern, inst)
	}
	return PatternState{}
}

func (*automatonBuilder) accept(*resyntax.Inst) PatternState { return PatternState{Op: PatternAccept} }

// fail is a step over no code point: the program of a pattern that matches nothing.
func (*automatonBuilder) fail(*resyntax.Inst) PatternState { return PatternState{Op: PatternStep} }

func (b *automatonBuilder) split(inst *resyntax.Inst) PatternState {
	return PatternState{Op: PatternSplit, Out: b.state(int(inst.Out)), Alt: b.state(int(inst.Arg))}
}

// anchor keeps `^` and `$` of a pattern without `(?m`, the text's two ends.
func (b *automatonBuilder) anchor(inst *resyntax.Inst) PatternState {
	empty := inst.Arg
	var at PatternAt
	for _, e := range patternEmpty {
		if empty&e.op != 0 {
			at |= e.at
			empty &^= e.op
		}
	}
	if empty != 0 {
		return b.refuse(inst)
	}
	return PatternState{Op: PatternAnchor, Out: b.state(int(inst.Out)), At: at}
}

// runes is a class; Go's compiler leaves InstRune a single rune only when folded, which needs `(?i`, refused by E1904.
func (b *automatonBuilder) runes(inst *resyntax.Inst) PatternState {
	if inst.Arg&uint32(resyntax.FoldCase) != 0 {
		return b.refuse(inst)
	}
	return b.step(inst, slices.Clone(inst.Rune))
}

func (b *automatonBuilder) rune1(inst *resyntax.Inst) PatternState {
	return b.step(inst, []rune{inst.Rune[0], inst.Rune[0]})
}

func (b *automatonBuilder) any(inst *resyntax.Inst) PatternState {
	return b.step(inst, slices.Clone(anyRune))
}

// anyNotNL is `.`: one code point but `\n`.
func (b *automatonBuilder) anyNotNL(inst *resyntax.Inst) PatternState {
	return b.step(inst, slices.Clone(anyRuneNotNL))
}

func (b *automatonBuilder) step(inst *resyntax.Inst, runes []rune) PatternState {
	return PatternState{Op: PatternStep, Out: b.state(int(inst.Out)), Runes: runes}
}
