// Package viewgen writes the view model's exact bytes (spec/VIEWMODEL.md 12.1's J1-J3, J10;
// WIRE.md 7): api.ViewModel().JSON() and the compiler's `emit view` (VIEWMODEL.md 14, a later
// unit) both call Write, so their bytes are identical. Write walks the api/vm structs by
// reflection into a jsonsrc.Node tree (a struct's members in its declared field order, J2,
// which vm.gen.go already keeps per its schema; a map's keys sorted by byte order, J2's
// program-named objects; a `omitzero` member left out when its Go value is the zero value, J3,
// api/vm/doc.go), and hands the tree to jsonsrc.Format, the one WIRE.md 7.4 printer. A
// json.RawMessage member (a J10 value) is parsed the same way (jsonsrc.Parse) and grafted in;
// the builder writes every number's canonical text (WIRE.md 7.2), and Write only validates it,
// never reformats it.
package viewgen
