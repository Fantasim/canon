package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// transCtx is a translated template being checked: its key as written and the interpolation
// every error is reported at, as E1703 (I18N.md T2).
type transCtx struct {
	key string
	at  source.Span
}

// checkTranslations checks the entries of p's translation files (I18N.md §4, §7).
func (c *checker) checkTranslations(p *pkgState) {
	for _, f := range p.files {
		if f.FileKind != syntax.FileTranslation {
			continue
		}
		for _, e := range f.Entries {
			c.translation(p, f, e)
		}
	}
}

// translation resolves an entry's key segments and, for a template key, types each
// interpolation in the scope of the source item (I18N.md T1–T3). A plain key's interpolations
// are E1707 and an unknown key is E1702, i18n's.
func (c *checker) translation(p *pkgState, f *syntax.File, e *syntax.TranslationEntry) {
	c.plainText(e.Text)
	if e.Key == nil {
		return
	}
	scope, step := c.readKey(p, f, e)
	s, ok := e.Text.(*syntax.StringLit)
	if step && ok {
		c.stepTranslation(p, f, e, s)
	}
	if scope == nil || !ok {
		return
	}
	for _, part := range s.Parts {
		if part.Interp == nil {
			continue
		}
		env := scope.push()
		env.pkg, env.owner, env.scopeFile, env.file = p, nil, scope.scoped(), f
		env.trans = &transCtx{key: qualified(e.Key), at: f.Span(part.Interp)}
		c.interpolation(env, part.Interp)
	}
}

// stepTranslation is a translated `step` text: a bare `{index}` is an Int local of the entry,
// any other interpolation an E1703 (I18N.md T1, log-2026-09-28 U5 calls).
func (c *checker) stepTranslation(p *pkgState, f *syntax.File, e *syntax.TranslationEntry, s *syntax.StringLit) {
	var index *object
	for _, part := range s.Parts {
		in := part.Interp
		if in == nil {
			continue
		}
		if id := bareIndex(in); id != nil {
			if index == nil {
				index = c.magicName(p, f, e, indexMember, types.IntType)
			}
			c.info.Uses[id], c.info.Types[id] = index, types.IntType
			continue
		}
		at := f.Span(in)
		c.reported++
		diag.E1703.AtStep(at, qualified(e.Key), at).Report(p.bag)
	}
}
