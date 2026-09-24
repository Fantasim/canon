package integrity

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// badStates flags an unknown rule id or an invalid mode in state.tsv; no file is no finding,
// an unreadable one leaves the rule unmeasured.
func badStates(ctx *lane.Context) ([]finding, error) {
	data, err := os.ReadFile(ctx.Repo.Abs(repo.StateFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", repo.StateFile, err)
	}
	var out []finding
	for _, row := range repo.ReadStateRows(string(data)) {
		f := finding{Rule: ruleState, File: repo.StateFile, Line: row.Line, Detail: row.Rule}
		switch _, ok := rules.Lookup(row.Rule); {
		case !ok:
			f.Message = msgUnknownRule + row.Rule
		case !rules.ValidMode(rules.Mode(row.Mode)):
			f.Message = msgBadMode + strconv.Quote(row.Mode)
		default:
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// statesLoosened flags a rule whose effective mode is looser than it was at HEAD.
func statesLoosened(ctx *lane.Context) ([]finding, error) {
	old, ok, err := headText(ctx, repo.StateFile)
	if err != nil || !ok {
		return nil, err
	}
	prev := map[string]rules.Mode{}
	for _, row := range repo.ReadStateRows(old) {
		prev[row.Rule] = rules.Mode(row.Mode)
	}
	var out []finding
	for id, m := range ctx.Repo.States() {
		rl, known := rules.Lookup(id)
		if !known {
			continue
		}
		was, had := prev[id]
		if !had {
			was = rl.Mode
		}
		if rules.Strictness(m) < rules.Strictness(was) {
			out = append(out, finding{
				Rule: ruleBaselineGuard, File: repo.StateFile, Detail: id,
				Message: msgStateLoose + id + " " + string(was) + " -> " + string(m),
			})
		}
	}
	return out, nil
}
