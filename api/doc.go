// Package canon is the public Go API of the Canon compiler: the library behind the canon
// command, the language server and the studio.
//
// canon.go is the frozen contract. Its normative description, with every rule referenced
// in it (O1, S5, E14, …), is spec/API.md. Bodies are stubs until milestone M4 of
// spec/IMPLEMENTATION-PLAN.md; small helpers whose behaviour is fully fixed by the spec
// (value and op constructors, error methods) are implemented already.
package canon
