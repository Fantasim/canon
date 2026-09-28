package i18n

// dot separates key segments; underscorePrefix names an unnamed show line (VIEWMODEL.md G17).
const (
	dot              = "."
	underscorePrefix = "_"
	spaceText        = " "
)

// studioSuffixProp is the studio package's `units` table field I18N.md U1 translates; it has
// no grammar keyword (unlike the studio names of internal/syntax's StudioMenu, StudioUnits).
const studioSuffixProp = "suffix"

// Plain is a text with no interpolation; Template is title, subtitle, a show line's second
// text, a check message or a step text (I18N.md L5).
const (
	Plain Kind = iota
	Template
)
