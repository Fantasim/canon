package safego

import "context"

// SafeGo runs fn in a goroutine.
func SafeGo(ctx context.Context, fn func(context.Context)) {
	go fn(ctx)
}
