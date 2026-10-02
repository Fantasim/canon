package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/lsp"
)

// runLSP is `canon lsp`, the language server on stdin and stdout until exit; an exit without
// shutdown is 1, as the protocol asks, and a broken stream 1 with its error.
func runLSP(inv *invocation) int {
	if len(inv.args) > 0 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdLSP, errNoArgs))
	}
	err := lsp.Serve(inv.ctx, inv.stdin(), inv.env.Stdout)
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return inv.fail(err)
	case !errors.Is(err, lsp.ErrNoShutdown):
		writeLine(inv.env.Stderr, msgPrefix+err.Error())
	}
	return exitErrors
}
