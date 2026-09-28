// Command benchgen writes a benchmark project sized for compiler performance testing: entry
// files placed by `@files`, refs into a define header and a monster table, an asset field, a
// twin package reading the same shape of data with `load.dir`, and a view. The same seed and
// entry count always write the same bytes. -out must be absent or empty.
//
//	go run ./internal/testkit/cmd/benchgen -seed 1 -out bench/ [-n 7000]
package main
