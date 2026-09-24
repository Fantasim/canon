package format

// kind is the kind of a layout document: text, a line break when broken (line, softline),
// always (hardline), an own-line or trailing comment, a blank line, a multiline string, the
// combinators (concat, ifBreak, indent, group, flat), and rule A (rhs).
type kind uint8

// doc is a layout document. A comment's flag is a blank line before it and alt a comment on the
// line of the one before; rule A's flag is a space before the operator and alt a value whose
// brackets break.
type doc struct {
	kind   kind
	text   string
	kids   []*doc
	lines  []string
	hard   bool // a genuine forced line break; propagates via cat, indent, group, ifBreak, rhs, flat
	flag   bool
	alt    bool
	tight  bool
	bit    bool
	forced bool // a group's own single-line choice (§6.1); propagates like hard, but never past flat
}

var (
	lineDoc     = &doc{kind: kLine}
	softlineDoc = &doc{kind: kSoftline}
	hardlineDoc = &doc{kind: kHardline, hard: true}
	blankDoc    = &doc{kind: kBlank, hard: true}
	emptyDoc    = &doc{kind: kText}
	spaceDoc    = &doc{kind: kText, text: space}
)

func text(s string) *doc {
	if s == "" {
		return emptyDoc
	}
	return &doc{kind: kText, text: s}
}

// cat concatenates ds, nil ones left out.
func cat(ds ...*doc) *doc {
	out := &doc{kind: kConcat, kids: make([]*doc, 0, len(ds))}
	for _, d := range ds {
		if d == nil || d == emptyDoc {
			continue
		}
		out.kids = append(out.kids, d)
		out.hard = out.hard || d.hard
		out.forced = out.forced || d.forced
		out.bit = out.bit || d.bit
	}
	return out
}

func indent(ds ...*doc) *doc {
	d := cat(ds...)
	return &doc{kind: kIndent, kids: []*doc{d}, hard: d.hard, forced: d.forced, bit: d.bit}
}

// group is a group of ds, broken whatever its width when force is set or ds holds a forced group.
func group(force bool, ds ...*doc) *doc {
	d := cat(ds...)
	return &doc{kind: kGroup, kids: []*doc{d}, hard: d.hard, forced: force || d.forced, bit: d.bit}
}

// listGroup is the group of a brace list, which carries the list's single-line bit (§6.1).
func listGroup(single bool, ds ...*doc) *doc {
	g := group(!single, ds...)
	g.bit = true
	return g
}

// flat is an always-flat construct (type arguments, where clauses, FORMATTER.md §6.2).
func flat(d *doc) *doc { return &doc{kind: kFlat, kids: []*doc{d}, hard: d.hard, bit: d.bit} }

func ifBreak(broken, flat *doc) *doc {
	return &doc{
		kind: kIfBreak, kids: []*doc{broken, flat}, hard: broken.hard || flat.hard,
		forced: broken.forced || flat.forced, bit: broken.bit || flat.bit,
	}
}

// comment is an own-line comment; a tight one has the next token on its line.
func comment(n note, blank bool) *doc {
	return &doc{kind: kComment, text: n.text, flag: blank, alt: n.sameLine, tight: n.joined, hard: true}
}

func suffix(s string) *doc { return &doc{kind: kSuffix, text: s, hard: true} }

// rhs is rule A of FORMATTER.md §7.3 for "op value".
func rhs(op, value *doc, spaced, bracket bool) *doc {
	r := &doc{
		kind: kRHS, kids: []*doc{op, value}, flag: spaced, alt: bracket,
		hard: op.hard || value.hard, forced: op.forced || value.forced, bit: op.bit || value.bit,
	}
	return group(false, r)
}

func multiline(lines []string) *doc { return &doc{kind: kMultiline, lines: lines, hard: true} }

// join is ds with sep between them.
func join(ds []*doc, sep *doc) *doc {
	var out []*doc
	for i, d := range ds {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, d)
	}
	return cat(out...)
}
