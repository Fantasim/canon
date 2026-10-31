package progen_test

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	tiersRule     = "GRAMMAR.md §7.1 (tiers)"
	tiersTypeRule = "TYPES.md §3.1 (import upward)"
	tiersClose    = "\n}"
	// The texts of the examples' tiers block that each operator mutates once.
	tiersName        = "    shared:"
	tiersNameQuoted  = `    "shared":`
	tiersFirstData   = "data: [balance, "
	tiersFirstTwice  = "data: [balance, balance, "
	tiersPipeline    = "pipeline, "
	tiersTeamboard   = "teamboard,"
	tiersTeamboards  = "teamboards,"
	tiersIDs         = "ids: [features.tiers.ids, resource.vocab]"
	tiersData        = "data: [balance, features, game, guide, pipeline, resource, service, teamboard, telemetry]"
	tiersSwappedIDs  = "ids: [balance, features, game, guide, pipeline, resource, service, teamboard, telemetry]"
	tiersSwappedData = "data: [features.tiers.ids, resource.vocab]"
	tiersIDsPath     = "features.tiers.ids"
)

// tiersOperators insert the examples' tiers block, mutated, as the project's last entry (DECISIONS 380).
func tiersOperators() []operator {
	return []operator{
		{code: diag.E1017.Def().Code, rule: tiersRule + " name", sites: tiersBlockWith(tiersName, tiersNameQuoted)},
		{code: diag.E1017.Def().Code, rule: tiersRule + " duplicate", sites: tiersBlockWith(tiersFirstData, tiersFirstTwice)},
		{code: diag.E1018.Def().Code, rule: tiersRule + " unplaced", sites: tiersBlockWith(tiersPipeline, "")},
		{code: diag.E1018.Def().Code, rule: tiersRule + " unused", many: true, sites: tiersBlockWith(tiersTeamboard, tiersTeamboards)},
		{code: diag.E2007.Def().Code, rule: tiersTypeRule, loose: true, sites: tiersSwap},
	}
}

// tiersBlockWith inserts the examples' tiers block, the first `from` in it replaced by `to`, as the
// last entry of the project; the finding lies inside the block.
func tiersBlockWith(from, to string) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		block, at, ok := tiersAt(tg)
		if !ok || !strings.Contains(block, from) {
			return nil
		}
		return []progen.Site{site(insert(at, strings.Replace(block, from, to, 1)))}
	}
}

// tiersSwap puts the ids and the data lists in each other's tier, so that a package importing
// features.tiers.ids imports upward; the check runs on the clean importer.
func tiersSwap(tg target) []progen.Site {
	block, at, ok := tiersAt(tg)
	if !ok || !strings.Contains(block, tiersIDs) || !strings.Contains(block, tiersData) {
		return nil
	}
	swapped := strings.Replace(strings.Replace(block, tiersIDs, tiersSwappedIDs, 1), tiersData, tiersSwappedData, 1)
	s := site(insert(at, swapped))
	for _, t := range *tg.all {
		if t.file != nil && slices.ContainsFunc(t.file.Imports, func(imp *syntax.Import) bool { return text(t, imp.Path) == tiersIDsPath }) {
			s.Packages = []string{t.pkg}
			return []progen.Site{s}
		}
	}
	return nil
}

// tiersAt is the examples' tiers block and where it goes in the harness's project: before the
// project's closing brace. ok is false on any other target.
func tiersAt(tg target) (block string, at int, ok bool) {
	i := strings.LastIndex(string(tg.src), tiersClose)
	if !isProject(tg) || tg.tiers == "" || i < 0 {
		return "", 0, false
	}
	return tg.tiers, i + 1, true
}
