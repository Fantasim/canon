// Loads each argument, <kind>=<path>, with the generated Go demo.shop loaders: "shelves" a file
// through LoadShelves, "reload" a directory through Store.Reload. Prints one line per argument:
// the load error, or "ok". The Go twin of strict_main.cpp (TestLoaderParity).
package main

import (
	"fmt"
	"os"
	"strings"

	shop "example.com/parity/shop"
)

func main() {
	for _, arg := range os.Args[1:] {
		kind, path, ok := strings.Cut(arg, "=")
		var err error
		switch {
		case ok && kind == "shelves":
			_, err = shop.LoadShelves(path)
		case ok && kind == "reload":
			err = shop.Store.Reload(path)
		default:
			os.Exit(100)
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("ok")
	}
}
