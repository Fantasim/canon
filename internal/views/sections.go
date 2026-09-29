package views

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/views/layout"
)

// views is the `views` section (VIEWMODEL.md 12.4): a view per record and variant of `types`.
func (b *builder) views() { b.m.Views = layout.Section(b.layoutInput(), b.defs.Described()) }

func (b *builder) layoutInput() layout.Input {
	return layout.Input{Info: b.in.Program.Info, Index: b.index, Res: b.res, Texts: b.texts, Assets: b.roots}
}

// i18n is the `i18n` section (12.10): the source language's texts, every catalogue key; each
// other language's translation files, missing count and non-empty translations.
func (b *builder) i18n() {
	src := project.DefaultLanguage
	if len(b.in.Languages) > 0 {
		src = b.in.Languages[0]
	}
	res := b.in.I18N[b.pkg.Path]
	texts := map[string]string{}
	if res != nil {
		for _, e := range res.Catalogue.Entries {
			texts[e.Key] = e.Text
		}
	}
	langs := map[string]vm.Language{src: {Texts: texts}}
	for _, lang := range b.otherLanguages() {
		missing := len(texts)
		l := vm.Language{Files: []string{}, Missing: &missing, Texts: map[string]string{}}
		if res != nil && res.Languages[lang] != nil {
			tr := res.Languages[lang]
			missing = tr.Missing
			l.Files = append(l.Files, tr.Files...)
			slices.Sort(l.Files)
			maps.Copy(l.Texts, tr.Texts)
		}
		langs[lang] = l
	}
	b.m.I18N = vm.I18N{Source: src, Languages: langs}
}

// findings is the `findings` section (12.11, J15): the package's findings of phases 1-7 in F2
// order, each as API.md 4.2's object, a named check's translated messages with it.
func (b *builder) findings() {
	langs := b.otherLanguages()
	for i, l := range diag.Locate(b.in.Files, b.in.Findings) {
		f := finding(l)
		f.Messages = b.messages(b.in.Findings[i], langs)
		b.m.Findings = append(b.m.Findings, f)
	}
}

// messages are f's message in each of langs its check translates (J15, I18N.md B5); none for a
// finding of no named one-line check.
func (b *builder) messages(f diag.Finding, langs []string) map[string]string {
	if f.Check == "" || b.in.CheckRun == nil || len(langs) == 0 {
		return nil
	}
	c, self, ok := b.in.CheckRun(f)
	if !ok {
		return nil
	}
	return b.render.Messages(b.pkg.Path, c, self, langs)
}

// finding is one finding as the model writes it (API.md F5).
func finding(l diag.Located) vm.Finding {
	f := vm.Finding{
		Severity: l.Severity.String(), Code: string(l.Code), File: l.Loc.Path, Line: l.Loc.Line, Col: l.Loc.Col,
		EndLine: l.Loc.EndLine, EndCol: l.Loc.EndCol, Package: l.Package, Path: l.Path, Message: l.Message,
		Check: l.Check, Layer: l.Layer, MoreFrames: l.MoreFrames, Reads: l.Reads,
	}
	if l.Pointer != "" {
		f.Pointer = &l.Pointer
	}
	for _, r := range l.Related {
		f.Related = append(f.Related, vm.Related{File: r.Loc.Path, Line: r.Loc.Line, Col: r.Loc.Col, EndLine: r.Loc.EndLine, EndCol: r.Loc.EndCol, Note: r.Note})
	}
	for _, fr := range l.Stack {
		f.Stack = append(f.Stack, vm.Frame{Fn: fr.Fn, File: fr.Loc.Path, Line: fr.Loc.Line, Col: fr.Loc.Col, EndLine: fr.Loc.EndLine, EndCol: fr.Loc.EndCol})
	}
	return f
}
