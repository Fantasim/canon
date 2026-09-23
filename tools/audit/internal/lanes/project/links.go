package project

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// deadLinks checks every relative link of every Markdown file the audit sees.
func deadLinks(ctx *lane.Context) []finding.Finding {
	var out []finding.Finding
	for _, f := range ctx.Repo.FilesWithExt(mdExt) {
		data, err := os.ReadFile(ctx.Repo.Abs(f))
		if err == nil {
			out = append(out, deadLinksIn(ctx, f, string(data))...)
		}
	}
	return out
}

// deadLinksIn checks the links of one file's prose: code blocks and code spans hold code
// (Canon's `[T](1..)` reads like a link), not links.
func deadLinksIn(ctx *lane.Context, f, text string) []finding.Finding {
	var out []finding.Finding
	fenced := false
	for i, l := range strings.Split(text, lineBreak) {
		if fenceRe.MatchString(l) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, target := range extractLinks(codeSpanRe.ReplaceAllString(l, "")) {
			resolved := path.Clean(path.Join(path.Dir(f), target))
			if _, err := os.Stat(ctx.Repo.Abs(resolved)); err != nil {
				out = append(out, finding.Finding{
					Rule: ruleDeadLink, File: f, Line: i + 1, Detail: target,
					Message: fmt.Sprintf(deadLinkMessage, target, resolved),
				})
			}
		}
	}
	return out
}

func extractLinks(line string) []string {
	var out []string
	for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
		if t := cleanLinkTarget(m[1]); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// cleanLinkTarget drops an anchor-only or external link and strips a trailing # fragment,
// a "title" and a :line suffix from what is left.
func cleanLinkTarget(raw string) string {
	t, _, _ := strings.Cut(strings.TrimSpace(raw), " ")
	if t == "" || strings.HasPrefix(t, fragmentPrefix) || schemeRe.MatchString(t) {
		return ""
	}
	if i := strings.IndexByte(t, fragmentByte); i >= 0 {
		t = t[:i]
	}
	return lineSuffixRe.ReplaceAllString(t, "")
}
