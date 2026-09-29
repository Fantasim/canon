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

// positionTag names a plain-list element `#<n>` (VIEWMODEL.md S9).
const positionTag = "#"

// A case path `v=c/w=c2` (VIEWMODEL.md L18).
const (
	caseSep = "/"
	caseIs  = "="
)

// The view items heading a value (VIEWMODEL.md 3.6).
var (
	titleItem    = headItem{kind: syntax.KindViewTitle, word: syntax.WordTitle, render: (*render.Renderer).Title}
	subtitleItem = headItem{kind: syntax.KindViewSubtitle, word: syntax.WordSubtitle, render: (*render.Renderer).Subtitle}
)
