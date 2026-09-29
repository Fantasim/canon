package live

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/render"
)

// Line is a `show` line's source template, keyed Key (`T.show.<id>.text`) in Pkg (VIEWMODEL.md
// 5.8). Fallback is read from what Eval evaluated (V13): a Lines that breaks one of the three
// rules on the fields below leaves a nested Fallback false (log-2026-09-29 M4 U9).
type Line struct {
	Pkg      string
	Key      []string         // (i) render Key's template as live's translated picks it, else Template
	Template syntax.StrLit    // the source template
	Magic    render.Magic     // (ii) passed to Eval unchanged: the memo is keyed by it (magicOf)
	Eval     render.Evaluator // (iii) evaluate all through it, a ref's target title with render's Magic{ID: id(e)}
}
