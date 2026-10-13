package writelock_test

import (
	"context"
	"fmt"
	"os"

	"github.com/fantasim/canonlang/internal/writelock"
)

// A build of game.items that imports core: game.items exclusively, core shared (DECISIONS 352).
func ExampleAcquire() {
	dir, err := os.MkdirTemp("", "writelock")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)
	held, err := writelock.Acquire(context.Background(), dir, []writelock.Request{
		{Package: "game.items", Mode: writelock.Exclusive},
		{Package: "core", Mode: writelock.Shared},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer held.Release()
	fmt.Println(held.Holds("game.items", writelock.Exclusive), held.Holds("core", writelock.Shared), held.Holds("core", writelock.Exclusive))
	// Output: true true false
}
