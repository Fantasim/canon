package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typeNaming is the .canon file a typed literal is printed in: its package, and the file when it
// exists, whose imports name other packages; nil for a Source's text, which names a type bare.
type typeNaming struct {
	pkg  string
	file *syntax.File
}

// qualifier is the text before the name of type name of package pkg in s, as check/imports.go's
// bindImport binds it: none in pkg's own files, a Source, or where `{ name }` brings it; an import's
// alias or last segment and a dot else; false where s imports no such package.
func (s *typeNaming) qualifier(pkg, name string) (string, bool) {
	if s == nil || s.pkg == pkg {
		return "", true
	}
	if s.file == nil {
		return "", false
	}
	for _, imp := range s.file.Imports {
		switch {
		case strings.Join(partNames(imp.Path.Parts), dotSeg) != pkg:
		case len(imp.Names) > 0:
			if slices.ContainsFunc(imp.Names, func(n *syntax.Ident) bool { return n.Name == name }) {
				return "", true
			}
		case imp.Alias != nil:
			return imp.Alias.Name + dotSeg, true
		default:
			return lastSegment(pkg) + dotSeg, true
		}
	}
	return "", false
}

// typedName is the typed literal's name (`RB`, `d.RB`; API.md M7, DECISIONS 171, 475) record r takes
// at a dependent position p.at where a bare brace reads another branch; false for a bare form. A
// record name the file lacks is a *ValueError; one no typed literal states leaves r unread there.
func (p *canonPrinter) typedName(r *value.Record) (string, bool) {
	if !p.atDependent() || reads(p.at, r.T, r) {
		return "", false
	}
	rec, ok := present(r.T).Base().(*types.RecordType)
	if !ok || !typedForm(rec) {
		p.unreadAt()
		return "", false
	}
	q, named := p.names.qualifier(rec.Pkg, rec.Name)
	if !named && p.err == nil {
		p.err = &ValueError{Expected: r.T.String(), Got: r.CanonText(), Detail: detailUnnamed}
	}
	return q + rec.Name, named
}

// atDependent reports the position printed of a dependent declared type (TYPES.md 11.4).
func (p *canonPrinter) atDependent() bool {
	return p.at != nil && dependent(present(p.at))
}

// unreadAt notes the position printed one whose text Canon reads as another branch than the value's
// (DECISIONS 171, 378, 475): a case, a record no typed literal states; the first one is kept.
func (p *canonPrinter) unreadAt() {
	if p.unread == nil {
		p.unread = p.at
	}
}

// noteCollection notes v, a list or map of n items at a dependent position, unread where its bare
// `[…]` or `{…}` reads as another branch than v's own type (DECISIONS 171, 474); an empty one fits.
func (p *canonPrinter) noteCollection(v value.Value, n int) {
	if n > 0 && p.atDependent() && !dependent(present(v.Type())) && !reads(p.at, v.Type(), v) {
		p.unreadAt()
	}
}

// typedForm reports a branch a typed literal states, so Canon text reads whichever branch its record
// computes (API.md M7, V1; DECISIONS 475): a record that takes no parameters (TYPES.md 11.1).
func typedForm(b types.Type) bool {
	r, ok := present(b).Base().(*types.RecordType)
	return ok && len(r.Params) == 0
}

// statedTyped reports v a record Canon text states as a typed literal on its own branch.
func statedTyped(v value.Value) bool {
	r, ok := v.(*value.Record)
	return ok && typedForm(r.T)
}

// naming is the file the operation's Canon text goes in: under an edit layer, the layer's file
// of the root's package (W11), else the file stating the path; only the package for a new file.
func (x *opCtx) naming() *typeNaming {
	out := &typeNaming{pkg: x.res.root.pkg.Path}
	if x.a.env.EditLayer != "" {
		if files := layerFiles(x.res.root.pkg, x.a.env.EditLayer); len(files) > 0 {
			out.file = files[0]
		}
		return out
	}
	if len(x.j.cur) > 0 {
		out.file = x.j.cur[x.statedIndex()].file
	}
	return out
}

// newFileText is entry key, rec, as a new entry file of the root's package states it: with no
// import (API.md N5; DECISIONS 366), so a typed literal of another package's record is a ValueError.
func (x *opCtx) newFileText(key string, rec *value.Record) (string, error) {
	was := x.a.names
	x.a.names = &typeNaming{pkg: x.res.root.pkg.Path}
	defer func() { x.a.names = was }()
	return x.a.entryText(key, rec, false)
}

// handle runs the operation's handler, refused with the ValueError of an inverse whose text no
// operation applies (API.md E22; DECISIONS 466, 474), but for a Set kept in its typed literal (M6).
func (x *opCtx) handle() error {
	x.a.unread = nil
	if err := handlers[x.op.Kind](x); err != nil {
		return err
	}
	if x.op.Kind == OpSet && x.keptTyped(x.nw) {
		x.a.unread = nil
	}
	return x.a.takeUnread(x.j.editable().Mode)
}

// takeUnread is the ValueError of the inverse just printed whose text Canon reads as another
// branch, bound for a destination of mode m; nil for a JSON one, whose decoder reads the computed
// branch (WIRE.md 5.9). It is cleared.
func (a *applier) takeUnread(m Mode) error {
	err := a.unread
	a.unread = nil
	if err == nil || m == ModeJSON {
		return nil
	}
	return err
}
