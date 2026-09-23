package diag

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/fantasim/canonlang/internal/source"
)

// Format is the form Render writes: FormatText (API.md §4.4) or FormatJSON (§4.2).
type Format uint8

// RenderOptions choose the form and give the summary line its counts and duration.
type RenderOptions struct {
	Format   Format
	Summary  Summary
	Duration time.Duration
	Golden   bool // text form: the duration is written (…) (IMPLEMENTATION-PLAN.md §7.2)
}

// Render writes findings in F2 order, then the summary line (API.md §4.4, CLI.md §2.4).
func Render(w io.Writer, files Files, findings []Finding, opt RenderOptions) error {
	return Write(w, Locate(files, findings), opt)
}

// Write is Render over resolved findings, which it sorts in F2 order with a total tiebreak
// (every field), so the output never depends on the order it is given.
func Write(w io.Writer, findings []Located, opt RenderOptions) error {
	sorted := sortLocated(findings)
	var out string
	if opt.Format == FormatJSON {
		out = jsonForm(sorted, opt)
	} else {
		out = textForm(sorted, opt)
	}
	if _, err := io.WriteString(w, out); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	return nil
}

// textForm is every finding's block, a blank line after each, then the summary (F14).
func textForm(findings []Located, opt RenderOptions) string {
	var lines []string
	for i := range findings {
		lines = append(lines, textFinding(&findings[i])...)
		lines = append(lines, "")
	}
	lines = append(lines, summaryText(opt))
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(strings.TrimRight(l, space)) // F9: no line ends with a space
		sb.WriteString(lineBreak)
	}
	return sb.String()
}

// textFinding is the lines of one finding (API.md F9-F13).
func textFinding(f *Located) []string {
	header := f.Severity.String() + codeOpen + string(f.Code) + codeClose
	if f.Loc.Path != "" {
		header += gap + f.Loc.Path + locSep + strconv.Itoa(f.Loc.Line) + locSep + strconv.Itoa(f.Loc.Col)
	}
	lines := []string{header}
	prefix := ""
	if f.Path != "" {
		prefix = f.Path + pathSep
	}
	for i, l := range strings.Split(f.Message, lineBreak) {
		if i == 0 {
			l = prefix + l
		}
		lines = append(lines, gap+l)
	}
	if f.Layer != "" {
		lines = append(lines, gap+layerPrefix+f.Layer)
	}
	for _, r := range f.Related {
		loc := shortLoc(r.Loc)
		if loc != "" {
			loc = space + loc
		}
		lines = append(lines, gap+relatedWords+loc+noted(r.Note))
	}
	for _, fr := range f.Stack {
		lines = append(lines, gap+framePrefix+fr.Fn+noted(shortLoc(fr.Loc)))
	}
	if f.MoreFrames > 0 {
		lines = append(lines, gap+parenOpen+strconv.Itoa(f.MoreFrames)+moreFramesClose)
	}
	return lines
}

// shortLoc is `<file>:<line>`, or "" for no location (API.md F12, F13).
func shortLoc(l source.Location) string {
	if l.Path == "" {
		return ""
	}
	return l.Path + locSep + strconv.Itoa(l.Line)
}

// noted is ` (<text>)`, or nothing for an empty text (API.md F12, F13).
func noted(text string) string {
	if text == "" {
		return ""
	}
	return noteOpen + text + noteClose
}

// summaryText is the summary line (API.md F15).
func summaryText(opt RenderOptions) string {
	s := opt.Summary
	notShown := ""
	if n := s.notShown(); n > 0 {
		notShown = fmt.Sprintf(notShownFormat, n)
	}
	return fmt.Sprintf(summaryFormat, counted(s.Errors, nounError, errorsWord),
		counted(s.Warnings, nounWarning, warningsWord), counted(s.Packages, nounPackage, packagesWord),
		notShown, durationText(opt))
}

// counted is `<n> <noun>`, singular when n is exactly 1.
func counted(n int, singular, plural string) string {
	noun := plural
	if n == 1 {
		noun = singular
	}
	return strconv.Itoa(n) + space + noun
}

// durationText is the summary's duration, `(…)` in goldens (API.md F15).
func durationText(opt RenderOptions) string {
	if opt.Golden {
		return durationGolden
	}
	d := max(opt.Duration, 0)
	if d < time.Second {
		return fmt.Sprintf(millisFormat, d.Milliseconds())
	}
	tenths := int64(d / tenth)
	return fmt.Sprintf(secondsFormat, tenths/tenthsPerSecond, tenths%tenthsPerSecond)
}
