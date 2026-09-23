package main

import (
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Stderr))
}

// run reports that no command exists yet and returns the usage-error exit code.
func run(stderr io.Writer) int {
	_, _ = io.WriteString(stderr, noCommand)
	return exitUsage
}
