package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// patKey is one step of a pattern below its parent pattern: a root (no parent), a field, any
// key, index or entry, or a case (API.md R13).
type patKey struct {
	parent int
	mark   string
	name   string
}

// patterns interns the patterns of R13, each text built once.
type patterns struct {
	ids  map[patKey]int
	text []string
}

func newPatterns() patterns {
	return patterns{ids: map[patKey]int{}}
}

// id is the pattern of step mark name below parent, made when new.
func (p *patterns) id(parent int, mark, name string) int {
	k := patKey{parent: parent, mark: mark, name: name}
	if id, ok := p.ids[k]; ok {
		return id
	}
	text := mark + name
	if mark == caseOpen {
		text += caseClose
	}
	if parent != noPlace {
		text = p.text[parent] + text
	}
	id := len(p.text)
	p.text = append(p.text, text)
	p.ids[k] = id
	return id
}

// step is the pattern of seg read in container below parent: a record's field as is, any
// other step as `[*]`, a table entry's `.key` included (R13).
func (p *patterns) step(parent int, container value.Value, seg Seg) int {
	if _, isRec := container.(*value.Record); isRec && seg.Kind == SegField {
		return p.id(parent, string(fieldMark), seg.Name)
	}
	return p.id(parent, patternAny, "")
}

// withCase is pat followed by `<c>` when v is a variant value of case c (R13).
func (p *patterns) withCase(pat int, v value.Value) int {
	if rec, ok := v.(*value.Record); ok {
		if c, isCase := baseOf(rec.T).(*types.CaseType); isCase {
			return p.id(pat, caseOpen, c.Name)
		}
	}
	return pat
}

// frame is the value the walk reached at one depth of its path, and once known: its pattern
// without and with its case, its trie position (noPlace off the trie), the nearest node.
type frame struct {
	v     value.Value
	bare  int
	pat   int
	trie  int
	owner int
	known bool
}

// frames are the walk's current path, one frame per depth, known lazily: a frame is set when
// the walk visits it, before anything below it, so the frames above a visit are its path's.
type frames struct {
	path []frame
	trie *segTrie
	pats *patterns
}

// at sets depth d to v, unknown.
func (fs *frames) at(d int, v value.Value) {
	for len(fs.path) <= d {
		fs.path = append(fs.path, frame{})
	}
	fs.path[d] = frame{v: v}
}

// reach is the frame at the end of segs from r, every frame above it known (R12, R13).
func (fs *frames) reach(r rootRef, segs []Seg) frame {
	for k := 0; k <= len(segs); k++ {
		f := &fs.path[k]
		if f.known {
			continue
		}
		f.known = true
		if k == 0 {
			f.bare = fs.pats.id(noPlace, "", r.qualified())
			f.trie = fs.trie.find(noPlace, rootTrieSeg(r))
			f.owner = fs.trie.node[f.trie]
		} else {
			up := fs.path[k-1]
			f.bare = fs.pats.step(up.pat, up.v, segs[k-1])
			f.trie, f.owner = noPlace, up.owner
			if up.trie != noPlace {
				f.trie = fs.trie.find(up.trie, segs[k-1])
			}
			if f.trie != noPlace && fs.trie.node[f.trie] != noPlace {
				f.owner = fs.trie.node[f.trie]
			}
		}
		f.pat = fs.pats.withCase(f.bare, f.v)
	}
	return fs.path[len(segs)]
}
