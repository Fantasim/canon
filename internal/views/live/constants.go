package live

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/render"
)

const (
	fmtWrap = "live: %w"
	fmtLang = "%w: %q"
)

// The separators of relative paths and keys (API.md V9, V10).
const (
	groupSep = "#"
	dot      = "."
)

// Unnamed `show` line ids `_<n>` (VIEWMODEL.md G17); a plain-list element's name `#<n>` (S9).
const (
	unnamedShow = "_"
	positionTag = "#"
)

// A case path `v=c/w=c2` (VIEWMODEL.md L18).
const (
	caseSep = "/"
	caseIs  = "="
)

// columnText is the mode of a column whose cells the compiler renders (VIEWMODEL.md T8).
const columnText = "text"

// sharedTitle is how many elements share a title that S9 disambiguates.
const sharedTitle = 2

// The view items heading a value (VIEWMODEL.md 3.6).
var (
	titleItem    = headItem{kind: syntax.KindViewTitle, word: syntax.WordTitle, render: (*render.Renderer).Title}
	subtitleItem = headItem{kind: syntax.KindViewSubtitle, word: syntax.WordSubtitle, render: (*render.Renderer).Subtitle}
)
