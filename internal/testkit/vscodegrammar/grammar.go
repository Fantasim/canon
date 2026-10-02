package vscodegrammar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
)

type captureJSON struct {
	Name string `json:"name"`
}

type ruleJSON struct {
	Name          string                 `json:"name"`
	ContentName   string                 `json:"contentName"`
	Match         string                 `json:"match"`
	Begin         string                 `json:"begin"`
	End           string                 `json:"end"`
	Captures      map[string]captureJSON `json:"captures"`
	BeginCaptures map[string]captureJSON `json:"beginCaptures"`
	EndCaptures   map[string]captureJSON `json:"endCaptures"`
	Patterns      []*ruleJSON            `json:"patterns"`
	Include       string                 `json:"include"`
}

type grammarJSON struct {
	Schema     string               `json:"$schema"`
	Name       string               `json:"name"`
	ScopeName  string               `json:"scopeName"`
	Patterns   []*ruleJSON          `json:"patterns"`
	Repository map[string]*ruleJSON `json:"repository"`
}

// rule is a compiled TextMate rule: a match, a begin/end pair, or a container of patterns.
type rule struct {
	name, content          string
	match, begin, end      *regexp.Regexp
	caps, begCaps, endCaps map[int]string
	patterns               []*rule
	flat                   []*rule
	flatDone               bool
}

// Grammar is a compiled TextMate grammar.
type Grammar struct {
	scope string
	root  *rule
	repo  map[string]*ruleJSON
	memo  map[*ruleJSON]*rule
}

// Load compiles a tmLanguage JSON; a pattern RE2 rejects fails the load, which is what
// keeps the grammar inside the subset of doc.go.
func Load(path string) (*Grammar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read grammar: %w", err)
	}
	var gj grammarJSON
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&gj); err != nil {
		return nil, fmt.Errorf("parse grammar: %w", err)
	}
	g := &Grammar{scope: gj.ScopeName, repo: gj.Repository, memo: map[*ruleJSON]*rule{}}
	g.root = &rule{}
	for _, p := range gj.Patterns {
		r, err := g.resolve(p)
		if err != nil {
			return nil, err
		}
		g.root.patterns = append(g.root.patterns, r)
	}
	return g, nil
}

// resolve compiles one pattern entry, following an include to the repository or to $self.
func (g *Grammar) resolve(j *ruleJSON) (*rule, error) {
	if j.Include == "" {
		return g.compile(j)
	}
	if j.Include == includeSelf {
		return g.root, nil
	}
	target, ok := g.repo[strings.TrimPrefix(j.Include, includeLocal)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errUnknownRule, j.Include)
	}
	return g.compile(target)
}

func (g *Grammar) compile(j *ruleJSON) (*rule, error) {
	if r, ok := g.memo[j]; ok {
		return r, nil
	}
	r := &rule{name: j.Name, content: j.ContentName}
	g.memo[j] = r
	var err error
	for _, p := range []struct {
		dst **regexp.Regexp
		src string
	}{{&r.match, j.Match}, {&r.begin, j.Begin}, {&r.end, j.End}} {
		if *p.dst, err = compileOptional(p.src); err != nil {
			return nil, err
		}
	}
	r.caps = captureIndex(j.Captures)
	r.begCaps = captureIndex(firstNonNil(j.BeginCaptures, j.Captures))
	r.endCaps = captureIndex(firstNonNil(j.EndCaptures, j.Captures))
	for _, p := range j.Patterns {
		c, err := g.resolve(p)
		if err != nil {
			return nil, err
		}
		r.patterns = append(r.patterns, c)
	}
	return r, nil
}

func compileOptional(src string) (*regexp.Regexp, error) {
	if src == "" {
		return nil, nil
	}
	re, err := regexp.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("pattern %s: %w", strconv.Quote(src), err)
	}
	if err := checkAnchors(src); err != nil {
		return nil, fmt.Errorf("pattern %s: %w", strconv.Quote(src), err)
	}
	return re, nil
}

// checkAnchors accepts a ^ only as the very first element of the pattern: the engine tries an
// anchored pattern at column 0 only, so an anchor elsewhere would mean something else in VS Code.
func checkAnchors(src string) error {
	re, err := syntax.Parse(src, syntax.Perl)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	want := 0
	if re.Op == syntax.OpBeginText || (re.Op == syntax.OpConcat && re.Sub[0].Op == syntax.OpBeginText) {
		want = 1
	}
	if countAnchors(re) != want {
		return errMisplacedAnchor
	}
	return nil
}

func countAnchors(re *syntax.Regexp) int {
	n := 0
	if re.Op == syntax.OpBeginText || re.Op == syntax.OpBeginLine {
		n = 1
	}
	for _, s := range re.Sub {
		n += countAnchors(s)
	}
	return n
}

func firstNonNil(a, b map[string]captureJSON) map[string]captureJSON {
	if a != nil {
		return a
	}
	return b
}

func captureIndex(m map[string]captureJSON) map[int]string {
	out := map[int]string{}
	for k, v := range m { //canon:unordered a map built from a map
		if n, err := strconv.Atoi(k); err == nil {
			out[n] = v.Name
		}
	}
	return out
}

// flatten lists the concrete rules a pattern list stands for, containers expanded.
func (r *rule) flatten() []*rule {
	if r.flatDone {
		return r.flat
	}
	r.flatDone = true
	seen := map[*rule]bool{r: true}
	var walk func(ps []*rule)
	walk = func(ps []*rule) {
		for _, p := range ps {
			if p.match == nil && p.begin == nil {
				if !seen[p] {
					seen[p] = true
					walk(p.patterns)
				}
				continue
			}
			r.flat = append(r.flat, p)
		}
	}
	walk(r.patterns)
	return r.flat
}
