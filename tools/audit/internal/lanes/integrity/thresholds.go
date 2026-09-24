package integrity

import (
	"fmt"
	"path/filepath"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/threshold"
)

// thresholdsRaised flags every threshold whose value is above the base revision's, when the
// thresholds file belongs to the audited repo.
func thresholdsRaised(ctx *lane.Context) ([]finding, error) {
	rel, inRepo := repoRel(ctx.Repo.Root, ctx.LimitsFile)
	if ctx.LimitsFile == "" || !inRepo {
		return nil, nil
	}
	old, ok, err := headText(ctx, rel)
	if err != nil || !ok {
		return nil, err
	}
	var out []finding
	for _, r := range ctx.Limits.Raised(threshold.Rows(old)) {
		out = append(out, finding{
			Rule: ruleBaselineGuard, File: rel, Detail: r.Key,
			Message: fmt.Sprintf(msgThreshold, r.Key, r.Was, r.Now), Fix: fixThreshold,
		})
	}
	return out, nil
}

// repoRel is file relative to root, slash-separated; false when it lies outside root.
func repoRel(root, file string) (string, bool) {
	rel, err := filepath.Rel(root, file)
	if err != nil || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
