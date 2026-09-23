// Command diaggen generates the diagnostic registry of package diag from spec/ERRORS.md:
// internal/diag/codes.go and the constructor table of its test. It refuses the catalogue,
// writing nothing, on any violation of the generator contract written in spec/ERRORS.md,
// and with -runtime it checks the code and text pairs of the runtime helper texts.
//
// Run from the repository root:
//
//	go run ./internal/diag/cmd/diaggen [-out internal/diag] [-runtime internal/gen]
package main
