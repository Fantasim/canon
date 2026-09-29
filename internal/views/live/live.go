package live

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
)

// Input is what live view states are computed from: a checked program and its settled values.
type Input struct {
	Program *check.Program
	Studio  string                  // project.studio's package, "" for none
	I18N    map[string]*i18n.Result // every package's catalogue and translations; nil: none
	Force   encode.Force            // the settled lets: ref targets and entry counts; nil: none
	Layout  *project.Layout         // places the asset roots of resolved types; nil: as written
	Eval    render.Evaluator        // evaluates view expressions; nil: every one fails (X7)
	Methods render.MethodEvaluator  // evaluates view-named methods; nil: every one fails (X7)
	Bound   Bound                   // the arguments of applied records; nil: read in the form only
	// Languages are project.languages, the source first: a Lang naming the source is the source
	Languages []string
}

// Bound is the arguments an applied record keeps (TYPES.md 11.1), by parameter: eval's, to wire.
type Bound func(*value.Record) map[*types.Param]value.Value

// Evaluate is the live view state of at.Value (API.md 11.1, 11.2): its heads; the `when`, `show`
// lines and dependent types of its form; the headings of the collections it holds or is (V6).
// A Lang without in.Languages is ErrNoLanguages.
func Evaluate(ctx context.Context, in Input, at Target) (*Result, error) {
	lang := at.Lang
	switch {
	case lang != "" && len(in.Languages) == 0:
		return nil, fmt.Errorf(fmtLang, ErrNoLanguages, lang)
	case lang != "" && lang == in.Languages[0]:
		lang = ""
	}
	s := newSession(ctx, in, lang)
	root := verify.Root("")
	if rec, ok := at.Value.(*value.Record); ok {
		m := at.Magic
		if m.ID == nil {
			m.ID = render.ID(rec)
		}
		h := s.heads(rec, m, at.Name)
		s.out.Title, s.out.Subtitle, s.out.Preview = h.title, h.subtitle, h.preview
		applied, _ := rec.T.Base().(*types.AppliedRecord)
		s.form(&frame{rec: rec, magic: m, applied: applied}, root)
	} else {
		s.out.Title, s.out.Subtitle = Text{Value: at.Name, OK: true}, Text{OK: true}
		if at.Value != nil && isCollection(at.Value) && at.Value.Type() != nil {
			s.collection(at.Value, root, s.control(at))
		}
	}
	if s.err != nil {
		return nil, fmt.Errorf(fmtWrap, s.err)
	}
	return s.out, nil
}

// control is the control of the collection at the path: its field's (VIEWMODEL.md C1), else its type's.
func (s *session) control(at Target) vm.Control {
	if at.Field != nil && at.Decl != nil {
		return s.res.Field(at.Decl, at.Field)
	}
	return s.res.Value(nil, at.Value.Type())
}
