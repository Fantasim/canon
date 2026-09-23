package integrity

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// ignores reports malformed suppressions (ignore-reason) and their total (ignore-count).
func ignores(ctx *lane.Context) []finding {
	var out []finding
	total := 0
	for _, rel := range ctx.Repo.FilesWithExt(sourceExts...) {
		directives := lane.Directives(ctx, rel)
		total += len(directives) + toolIgnores(ctx, rel)
		if ctx.On(ruleReason) {
			out = append(out, badDirectives(rel, directives)...)
		}
	}
	if ctx.On(ruleCount) && total > 0 {
		out = append(out, finding{
			Rule: ruleCount, File: repoScope, Value: total,
			Message: strconv.Itoa(total) + " ignore directives in the repo",
		})
	}
	return out
}

// toolIgnores counts the other Go tools' suppressions in rel's comments; Markdown has none.
func toolIgnores(ctx *lane.Context, rel string) int {
	if !strings.HasSuffix(rel, repo.GoExt) {
		return 0
	}
	text := commentText(ctx, rel)
	n := 0
	for _, m := range ignoreMarkers {
		n += strings.Count(text, m)
	}
	return n
}

// commentText is rel's comments only, line numbers kept: a marker named in a string or in
// code (this tool's own constants) is not an ignore.
func commentText(ctx *lane.Context, rel string) string {
	if strings.HasSuffix(rel, repo.GoExt) && ctx.Go != nil {
		return ctx.Go.CommentLines(rel)
	}
	src, err := os.ReadFile(ctx.Repo.Abs(rel))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(src), "\n")
	for i, l := range lines {
		lines[i] = commentPart(l)
	}
	return strings.Join(lines, "\n")
}

func commentPart(line string) string {
	if strings.HasPrefix(strings.TrimSpace(line), docLineMark) {
		return line
	}
	at := -1
	for _, open := range commentOpeners {
		if i := strings.Index(line, open); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	if at < 0 {
		return ""
	}
	return line[at:]
}

func badDirectives(rel string, ds []lane.Directive) []finding {
	var out []finding
	for _, d := range ds {
		f := finding{Rule: ruleReason, File: rel, Line: d.Line, Detail: strings.Join(d.Rules, ",")}
		unknown := slices.IndexFunc(d.Rules, func(id string) bool { _, ok := rules.Lookup(id); return !ok })
		switch {
		case !d.Valid():
			f.Message = msgNoReason
		case unknown >= 0:
			f.Message = msgUnknownIgn + d.Rules[unknown]
		default:
			continue
		}
		out = append(out, f)
	}
	return out
}
