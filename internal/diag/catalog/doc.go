// Package catalog reads spec/ERRORS.md, the single source of Canon's diagnostics, validates
// it against the generator contract written there, and generates the registry of package
// diag. It also reads the package table of spec/IMPLEMENTATION-PLAN.md, which names the
// owning packages and orders the dependency rule.
package catalog
