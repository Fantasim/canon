package grammar

import (
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Kind is a kind of Canon file the generator writes (GRAMMAR.md §5.2).
type Kind uint8

// Kinds are every Kind, in order.
func Kinds() []Kind { return []Kind{Source, Layer, Translation, Project} }

// File is the project-relative path of a file of kind k (I18N.md §4).
func (k Kind) File() string {
	switch k {
	case Layer:
		return genDir + genStem + layerExt + project.SourceExt
	case Translation:
		return genDir + genStem + dotSep + langCodes[0] + project.SourceExt
	case Project:
		return project.FileName
	default:
		return genDir + genStem + project.SourceExt
	}
}

// Syntax is the syntax.FileKind the parser reads a file of kind k as.
func (k Kind) Syntax() syntax.FileKind {
	if k == Project {
		return syntax.FileProject
	}
	return syntax.FileSource
}

// String names the kind.
func (k Kind) String() string {
	return [kindCount]string{Source: sourceWord, Layer: syntax.KwLayer.String(), Translation: syntax.KwTranslation.String(), Project: syntax.KwProject.String()}[k]
}
