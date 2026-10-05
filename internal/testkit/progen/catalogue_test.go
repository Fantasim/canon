package progen_test

// catalogue is every mutation operator, in code order within each range.
func catalogue() []operator {
	var out []operator
	for _, part := range [][]operator{projectOperators(), syntaxOperators(), namesOperators(), typesOperators(), valuesOperators(), evalOperators(), wireOperators(), emitOperators(), loadOperators(), loadAddOperators(), viewsOperators(), viewBasicsOperators(), viewPropsOperators(), viewGroupsOperators(), viewTargetsOperators(), viewStudioOperators(), i18nOperators(), ergonomicsOperators()} {
		out = append(out, part...)
	}
	return out
}
