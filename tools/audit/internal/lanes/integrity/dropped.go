package integrity

import (
	"strconv"
	"strings"

	fpkg "github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// droppedBlock is one run of removed comment lines in a diff.
type droppedBlock struct {
	file  string
	line  int
	lines []string
}

// decisionsDropped is decision-dropped: a comment recording a decision removed since the base,
// its wording re-added nowhere, while no decision record changed. An error means the diff
// itself could not be read; the caller decides whether that is a real failure worth a Skip.
func decisionsDropped(ctx *lane.Context) ([]finding, error) {
	git := ctx.Repo.Git
	base := ctx.Repo.Base()
	diff, err := git(gitDiff, gitNoColor, gitRelative, gitUnifiedZero, base)
	if err != nil {
		return nil, err
	}
	if recordsChanged(git, diff) || declaredObsolete(git, base) {
		return nil, nil
	}
	removed, added := parseDiff(diff)
	var out []finding
	for _, b := range removed {
		if phrase := droppedDecision(b, added[b.file]); phrase != "" {
			out = append(out, finding{
				Rule: ruleDecision, File: b.file, Line: b.line, Value: len(b.lines),
				Detail:  fpkg.Normalize(phrase),
				Message: msgDecision + phrase,
			})
		}
	}
	return out, nil
}

// droppedDecision is the removed line carrying b's decision wording when the file's added
// comments do not carry that wording again (a reflow keeps it), else "".
func droppedDecision(b droppedBlock, added string) string {
	text := squash(strings.Join(b.lines, " "))
	loc := reDecision.FindStringIndex(text)
	if loc == nil || strings.Contains(squash(added), decisionContext(text, loc)) {
		return ""
	}
	marker := text[loc[0]:loc[1]]
	for _, l := range b.lines {
		if strings.Contains(squash(l), marker) {
			return strings.TrimSpace(l)
		}
	}
	return strings.TrimSpace(b.lines[0])
}

// decisionContext is the marker and the words after it, enough to tell a reflow from a cut.
func decisionContext(text string, loc []int) string {
	words := strings.Fields(text[loc[0]:])
	return strings.Join(words[:min(len(words), decisionContextWords)], " ")
}

// squash lowercases s and drops comment markers and line breaks, so wrapped text compares equal.
func squash(s string) string {
	s = strings.NewReplacer("//", " ", "/*", " ", "*/", " ", "*", " ").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// recordsChanged reports an edit to a decision record, committed or not, tracked or not.
func recordsChanged(git func(...string) (string, error), diff string) bool {
	for l := range strings.SplitSeq(diff, "\n") {
		if isRecord(strings.TrimPrefix(l, diffFilePrefix)) && strings.HasPrefix(l, diffFilePrefix) {
			return true
		}
	}
	status, err := git(gitStatus, gitPorcelain, repo.DecisionsFile)
	return err == nil && strings.TrimSpace(status) != ""
}

func isRecord(path string) bool { return path == repo.DecisionsFile }

// declaredObsolete reports a commit since base whose message marks the dropped decisions void.
func declaredObsolete(git func(...string) (string, error), base string) bool {
	msgs, err := git(gitLog, gitMessageFormat, base+gitRange+repo.DefaultBase)
	return err == nil && strings.Contains(msgs, decisionObsolete)
}

// parseDiff splits a --unified=0 diff of source files into removed comment runs and, per file,
// the text of every added comment line.
func parseDiff(diff string) ([]droppedBlock, map[string]string) {
	var blocks []droppedBlock
	added := map[string]string{}
	file, line := "", 0
	var cur *droppedBlock
	for l := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(l, diffFilePrefix):
			file, cur = strings.TrimPrefix(l, diffFilePrefix), nil
		case strings.HasPrefix(l, diffHunkPrefix):
			line, cur = hunkStart(l), nil
		case strings.HasPrefix(l, diffOldHeader) || !isSource(file):
			continue
		case strings.HasPrefix(l, diffRemoved):
			cur = extendBlock(&blocks, cur, file, line, commentPart(l[1:]))
			line++
		case strings.HasPrefix(l, diffAdded):
			added[file] += commentPart(l[1:]) + "\n"
		}
	}
	return blocks, added
}

// extendBlock appends a removed comment line to the current run, opening one when needed; a
// removed code line ends the run.
func extendBlock(blocks *[]droppedBlock, cur *droppedBlock, file string, line int, comment string) *droppedBlock {
	if comment == "" {
		return nil
	}
	if cur == nil {
		*blocks = append(*blocks, droppedBlock{file: file, line: line})
		cur = &(*blocks)[len(*blocks)-1]
	}
	cur.lines = append(cur.lines, comment)
	return cur
}

func hunkStart(header string) int {
	g := reHunkOld.FindStringSubmatch(header)
	if g == nil {
		return 0
	}
	n, _ := strconv.Atoi(g[1])
	return n
}

// isSource reports a Go file: the only comments decision-dropped judges.
func isSource(path string) bool { return strings.HasSuffix(path, repo.GoExt) }
