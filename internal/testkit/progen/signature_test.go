package progen_test

import (
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// A signature names a failure by what moves with neither the site nor shrinking: its kind, each
// finding's code and message variant, or an error's text, its places cut.
var (
	rePlace       = regexp.MustCompile(`[\w./@-]+:\d+(:\d+)?`)
	rePlaceholder = regexp.MustCompile(`\{\{|\}\}|\{[A-Za-z]+\}`)
	variantsOnce  sync.Once
	variantsVal   map[diag.Code][]variantRe
)

// variantRe is a message variant and the regular expression its template renders to.
type variantRe struct {
	name string
	re   *regexp.Regexp
}

// variants are the message variants of every code, as regular expressions.
func variants() map[diag.Code][]variantRe {
	variantsOnce.Do(func() {
		variantsVal = map[diag.Code][]variantRe{}
		for _, d := range diag.Registry {
			for _, v := range d.Variants {
				variantsVal[d.Code] = append(variantsVal[d.Code], variantRe{v.Name, templateRe(v.Template)})
			}
		}
	})
	return variantsVal
}

// templateRe matches the messages a template renders: "{{" and "}}" are braces, "{arg}" any text.
func templateRe(tpl string) *regexp.Regexp {
	var b strings.Builder
	last := 0
	for _, m := range rePlaceholder.FindAllStringIndex(tpl, -1) {
		b.WriteString(regexp.QuoteMeta(tpl[last:m[0]]))
		switch tpl[m[0]:m[1]] {
		case "{{":
			b.WriteString(regexp.QuoteMeta("{"))
		case "}}":
			b.WriteString(regexp.QuoteMeta("}"))
		default:
			b.WriteString("(?s:.*)")
		}
		last = m[1]
	}
	b.WriteString(regexp.QuoteMeta(tpl[last:]))
	return regexp.MustCompile("^(?s:" + b.String() + ")$")
}

// shapeOf is a finding's code and message variant, "E1905/key", or its code alone.
func shapeOf(code diag.Code, message string) string {
	for _, v := range variants()[code] {
		if v.name != "" && v.re.MatchString(message) {
			return string(code) + "/" + v.name
		}
	}
	return string(code)
}

// shapes are the findings' shapes, sorted, each once.
func shapes(fs []progen.Finding) string {
	var out []string
	for _, f := range fs {
		out = append(out, shapeOf(f.Code, f.Message))
	}
	slices.Sort(out)
	return strings.Join(slices.Compact(out), ", ")
}

// compilerFrames are the first distinct compiler functions of a panic's stack, innermost first.
func compilerFrames(stack string) string {
	names := frameNames(stack, sigFrames)
	return strings.Join(names, " < ")
}

// recursion names a stack overflow by the sorted compiler functions its innermost frames repeat,
// whichever of them the stack ran out in; by all their names when none repeats.
func recursion(stack string) string {
	names := frameList(stack, cycleFrames)
	var repeated []string
	for i, name := range names {
		if slices.Contains(names[i+1:], name) && !slices.Contains(repeated, name) {
			repeated = append(repeated, name)
		}
	}
	if len(repeated) == 0 {
		repeated = slices.Clone(names)
	}
	slices.Sort(repeated)
	return strings.Join(slices.Compact(repeated), cycleSep)
}

// frameNames are the first n distinct compiler functions of a stack.
func frameNames(stack string, n int) []string {
	var out []string
	for _, name := range frameList(stack, len(stack)) {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
		if len(out) == n {
			break
		}
	}
	return out
}

// frameList are the compiler functions of a stack's first window compiler frames, in order,
// none past the line where Go elides the middle of a deep stack.
func frameList(stack string, window int) []string {
	var out []string
	for _, line := range strings.Split(stack, "\n") {
		if len(out) == window || strings.HasPrefix(line, elidedMark) {
			break
		}
		name, ok := strings.CutPrefix(line, compilerPkg)
		if !ok || strings.HasPrefix(name, harnessPkg) {
			continue
		}
		if i := strings.LastIndexByte(name, '('); i > 0 {
			name = name[:i]
		}
		out = append(out, name)
	}
	return out
}

// sigKey is a signature as an archive header holds it: its words, one space apart.
func sigKey(sig string) string { return strings.Join(strings.Fields(sig), " ") }

// unplaced is an error text without the places it names, each line once, sorted.
func unplaced(msg string) string {
	var out []string
	for _, line := range strings.Split(msg, "\n") {
		head, _, _ := strings.Cut(line, " at ")
		out = append(out, strings.TrimSpace(rePlace.ReplaceAllString(head, "_")))
	}
	slices.Sort(out)
	return strings.Join(slices.Compact(out), "; ")
}

// seen maps each suite, name and signature this process met to the seed that first showed it.
var seen = map[string]uint64{}

// reported fails t for a failure and tells whether it is done with it: an open archive stands
// for it (logged), its signature was already met in this run (one line), or it is not to be
// kept. Otherwise the caller shrinks it and reports it.
func reported(t *testing.T, suite, name string, seed uint64, v verdict) bool {
	t.Helper()
	key := suite + " " + name + " " + sigKey(v.Sig)
	first, met := seen[key]
	switch {
	case openBug(suite, name, v.Sig):
		t.Logf("known open bug (%s %s): %s", suite, name, v.Sig)
	case met:
		fail(t, "%s %s seed %d: the failure of seed %d (%s)", suite, name, seed, first, v.Sig)
	case !*flagKeep || alreadyKept(suite, name, v.Sig):
		seen[key] = seed
		fail(t, "%s %s seed %d: %s\nsignature %s\n%s(-progen.keep shrinks and keeps it under %s)", suite, name, seed, v.Text, v.Sig, v.Detail, keptDir)
	default:
		seen[key] = seed
		return false
	}
	return true
}
