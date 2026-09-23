package lane

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// Run drives every lane that produces an enabled rule, one after the other (memory budget),
// then drops what the rules or ignore directives exclude, names Go symbols, fills defaults.
func Run(ctx *Context, lanes []Lane) ([]finding.Finding, []Skip) {
	var (
		all   []finding.Finding
		skips []Skip
	)
	for _, l := range lanes {
		if !wanted(ctx, l) {
			continue
		}
		start := time.Now()
		res, err := l.Run(ctx)
		ctx.Logf("lane %s: %d findings in %s", l.Name(), len(res.Findings), time.Since(start).Round(time.Millisecond))
		all = append(all, res.Findings...)
		skips = append(skips, res.Skipped...)
		if err != nil {
			skips = append(skips, Skip{What: "lane " + l.Name(), Reason: err.Error()})
		}
	}
	return post(ctx, all), skips
}

func wanted(ctx *Context, l Lane) bool {
	for _, id := range l.Rules() {
		if ctx.On(id) {
			return true
		}
	}
	return false
}

func post(ctx *Context, in []finding.Finding) []finding.Finding {
	ign := newIgnores(ctx)
	var out []finding.Finding
	for _, f := range in {
		if !ctx.On(f.Rule) || ctx.generated(f.File) || ign.suppressed(f) {
			continue
		}
		if f.Symbol == "" && ctx.Go != nil && strings.HasSuffix(f.File, repo.GoExt) && f.Line > 0 {
			f.Symbol = ctx.Go.Enclosing(f.File, f.Line)
		}
		if rl, ok := rules.Lookup(f.Rule); ok {
			if f.Fix == "" {
				f.Fix = rl.Fix
			}
			if f.Message == "" {
				f.Message = rl.Describe(ctx.Limits)
			}
		}
		f.Detail = finding.Clean(f.Detail)
		out = append(out, f)
	}
	sortFindings(out)
	return out
}

// sortFindings orders findings by rulebook order, then file, then line.
func sortFindings(fs []finding.Finding) {
	idx := map[string]int{}
	for i, rl := range rules.All {
		idx[rl.ID] = i
	}
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if idx[a.Rule] != idx[b.Rule] {
			return idx[a.Rule] < idx[b.Rule]
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Detail < b.Detail
	})
}

func (c *Context) generated(rel string) bool {
	if c.Go == nil || !strings.HasSuffix(rel, repo.GoExt) {
		return false
	}
	f, ok := c.Go.File(rel)
	return ok && f.Generated
}

// Directive is one `sovaudit:ignore` found in a file.
type Directive struct {
	File   string
	Line   int
	Rules  []string
	Reason string
	Whole  bool
}

func (d Directive) Valid() bool { return len(d.Rules) > 0 && d.Reason != "" }

// parseDirectives finds the sovaudit:ignore directives of a file: a comment whose text
// starts with the marker. In Go, pass only comment text (see CommentLines).
func parseDirectives(rel string, src []byte) []Directive {
	return parseWith(reDirective, rel, src)
}

func parseWith(re *regexp.Regexp, rel string, src []byte) []Directive {
	var out []Directive
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, scanBuf), scanBuf)
	for n := 1; sc.Scan(); n++ {
		if d, ok := parseDirective(re, rel, n, sc.Text()); ok {
			out = append(out, d)
		}
	}
	return out
}

func parseDirective(re *regexp.Regexp, rel string, n int, line string) (Directive, bool) {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return Directive{}, false
	}
	d := Directive{File: rel, Line: n, Whole: m[directiveWholeGroup] != ""}
	spec, reason, _ := strings.Cut(m[directiveSpecGroup], reasonSep)
	d.Reason = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(reason), htmlClose))
	d.Rules = strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' })
	return d, true
}

// Directives reads rel's directives: from Go comments only (a directive inside a string
// literal is not one), from a line-opening HTML comment in Markdown, from comment-opened
// text elsewhere.
func Directives(ctx *Context, rel string) []Directive {
	src, err := os.ReadFile(ctx.Repo.Abs(rel))
	if err != nil {
		return nil
	}
	if strings.HasSuffix(rel, repo.GoExt) && ctx.Go != nil {
		return parseWith(reGoDirective, rel, []byte(ctx.Go.CommentLines(rel)))
	}
	if strings.HasSuffix(rel, repo.MarkdownExt) {
		return parseWith(reMdDirective, rel, src)
	}
	return parseDirectives(rel, src)
}

type ignores struct {
	ctx   *Context
	cache map[string][]Directive
}

func newIgnores(ctx *Context) *ignores { return &ignores{ctx: ctx, cache: map[string][]Directive{}} }

func (ig *ignores) suppressed(f finding.Finding) bool {
	file := f.File
	if f.Src != "" {
		file = f.Src
	}
	ds, ok := ig.cache[file]
	if !ok {
		ds = Directives(ig.ctx, file)
		ig.cache[file] = ds
	}
	for _, d := range ds {
		if !d.Valid() || !slices.Contains(d.Rules, f.Rule) {
			continue
		}
		if d.Whole || f.Line == d.Line || f.Line == d.Line+1 {
			return true
		}
	}
	return false
}
