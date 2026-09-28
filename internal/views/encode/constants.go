package encode

import "github.com/fantasim/canonlang/internal/syntax"

// Separators of qualified names, value ids and text keys (VIEWMODEL.md J7–J9).
const (
	dot   = "."
	colon = ":"
)

// The element of a define table and the key types of a ref (VIEWMODEL.md J12).
const (
	elemDefine = "$define"
	keyInt     = "int"
	keyString  = "string"
)

// The JSON pieces of the value encoding (VIEWMODEL.md J10).
const (
	valueNull   = "null"
	keyCase     = "$case"
	openArray   = '['
	closeArray  = ']'
	openObject  = '{'
	closeObject = '}'
	comma       = ','
	colonByte   = ':'
)

// The kind words of translation keys (I18N.md K4).
const (
	segField  = "field"
	segMethod = "method"
	segCase   = "case"
	segMember = "member"
)

// The text parts of translation keys that are not view properties (I18N.md §3.2).
const (
	segTitle    = "title"
	segSubtitle = "subtitle"
	segSingular = "singular"
	segPlural   = "plural"
	segGroup    = "group"
	segShow     = "show"
	segCheck    = "check"
	segIntro    = "intro"
	segText     = "text"
	Deprecated  = "deprecated" // a field's deprecation reason (I18N.md §3.3)
)

// reserved are the reserved segments of translation keys (I18N.md §3.2, K4).
var reserved = map[string]bool{
	syntax.PropHelp: true, segTitle: true, segSubtitle: true, segSingular: true, segPlural: true,
	segGroup: true, segShow: true, segCheck: true, segIntro: true, segText: true,
	Deprecated: true, syntax.PropPlaceholder: true, syntax.PropNone: true, syntax.PropStep: true,
	segField: true, segMethod: true, segCase: true, segMember: true,
}
