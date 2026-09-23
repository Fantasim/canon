package integrity

import (
	"fmt"
	"path/filepath"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// thresholdsRaised flags every threshold whose value is above the base revision's, when the
// thresholds file belongs to the audited repo.
func thresholdsRaised(ctx *lane.Context) []finding {
	rel, err := filepath.Rel(ctx.Repo.Root, ctx.LimitsFile)
	if ctx.LimitsFile == "" || err != nil || !filepath.IsLocal(rel) {
		return nil
	}
	rel = filepath.ToSlash(rel)
	old, ok := headText(ctx, rel)
	if !ok {
		return nil
	}
	var out []finding
	for _, r := range ctx.Limits.Raised(threshold.Rows(old)) {
		out = append(out, finding{
			Rule: ruleBaselineGuard, File: rel, Detail: r.Key,
			Message: fmt.Sprintf(msgThreshold, r.Key, r.Was, r.Now), Fix: fixThreshold,
		})
	}
	return out
}
