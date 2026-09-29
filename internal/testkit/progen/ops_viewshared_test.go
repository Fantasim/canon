package progen_test

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// emitViewTarget is the name emit targets that produce a view model (VIEWMODEL.md §12.1).
const emitViewTarget = "view"

// appendBlock is a site appending a whole new construct at the end of tg's file (DECISIONS 200).
func appendBlock(tg target, block string) progen.Site {
	return site(insert(declEnd(tg), "\n\n"+block+"\n"))
}

// packageHasEmitView tells tg's package declaring "emit view" somewhere (I18N.md W1).
func packageHasEmitView(tg target) bool {
	for _, p := range peers(tg) {
		for _, e := range nodes[*syntax.EmitDecl](p) {
			if e.Target != nil && e.Target.Name == emitViewTarget {
				return true
			}
		}
	}
	return false
}

// firstOfPackage tells tg the first target of its package: one insertion per package, not per file.
func firstOfPackage(tg target) bool {
	own := peers(tg)
	return len(own) > 0 && own[0].path == tg.path
}
