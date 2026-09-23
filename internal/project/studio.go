package project

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// CheckStudio reports E1012 when `studio` names no package of units (GRAMMAR.md §7.1).
func CheckStudio(p *Project, units []*Unit, bag *diag.Bag) {
	if p.Studio.Path == "" || slices.ContainsFunc(units, func(u *Unit) bool { return u.Name == p.Studio.Path }) {
		return
	}
	diag.E1012.At(p.Studio.Span, p.Studio.Path).Report(bag)
}
