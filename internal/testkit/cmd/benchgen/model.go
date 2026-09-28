package main

import "fmt"

// itemCode, categoryCode and monsterCode are the padded, 0-based identifiers of each table.
func itemCode(index int) string     { return fmt.Sprintf(itemCodeFmt, index+1) }
func categoryCode(index int) string { return fmt.Sprintf(categoryCodeFmt, index+1) }
func monsterCode(index int) string  { return fmt.Sprintf(monsterCodeFmt, index+1) }

// genModel is the whole benchmark project's shape, sized from n.
type genModel struct {
	categories []string
	refPool    []string
	monsters   []string
	cases      []string
}

// buildModel sizes every collection from n.
func buildModel(n int) *genModel {
	monsterCount := max(1, n/monsterDivisor)
	poolSize := max(1, n/categoryGroupDivisor)
	m := &genModel{
		categories: make([]string, n),
		monsters:   make([]string, monsterCount),
		cases:      caseNames(),
	}
	for i := range m.categories {
		m.categories[i] = categoryCode(i)
	}
	for i := range m.monsters {
		m.monsters[i] = monsterCode(i)
	}
	m.refPool = m.categories[:poolSize]
	return m
}
