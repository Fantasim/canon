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
	keyed := sortFindings(files, findings)
	var out string
	if opt.Format == FormatJSON {
		out = jsonForm(files, keyed, opt)
	} else {
		out = textForm(files, keyed, opt)
	}
	if _, err := io.WriteString(w, out); err != nil {
		return fmt.Errorf("%w: %w", ErrWrite, err)
	}
	return nil
}

// textForm is every finding's block, a blank line after each, then the summary (F14).
func textForm(files Files, keyed []keyedFinding, opt RenderOptions) string {
	var lines []string
	for _, k := range keyed {
		lines = append(lines, textFinding(files, k)...)
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
func textFinding(files Files, k keyedFinding) []string {
	f := k.f
	header := f.Severity.String() + codeOpen + string(f.Code) + codeClose
	if k.path != "" {
		header += gap + k.path + locSep + strconv.Itoa(k.line) + locSep + strconv.Itoa(k.col)
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
		loc := shortLoc(files, r.Span)
		if loc != "" {
			loc = space + loc
		}
		lines = append(lines, gap+relatedWords+loc+noted(r.Note))
	}
	for _, fr := range f.Stack {
		lines = append(lines, gap+framePrefix+fr.Fn+noted(shortLoc(files, fr.Span)))
	}
	if f.MoreFrames > 0 {
		lines = append(lines, gap+parenOpen+strconv.Itoa(f.MoreFrames)+moreFramesClose)
	}
	return lines
}

// shortLoc is `<file>:<line>`, or "" for a span without a file.
func shortLoc(files Files, s source.Span) string {
	path := files.Path(s.File)
	if path == "" {
		return ""
	}
	return renderer{files: files}.loc(s)
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
