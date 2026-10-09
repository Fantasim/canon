package gogen

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// openMember is one member of an opened enum as its templates write it: its constant, its Canon name quoted, its code.
type openMember struct {
	Const, Text, Number string
}

// openView is the data of the openEnum and openCodes templates.
type openView struct {
	Name, String, Wire, Known, Code, Parse, FromCode, Arg, Codes, Consts string
	Members                                                              []openMember
}

// writeOpen writes an opened enum's methods after its `string` type and wire-valued constants: String (the Canon name of a known member, else the wire), Wire, Known, Parse<E> (E(wire) and false for an unknown one), <E>Members, and with @codes Code and <E>FromCode, each false for an unknown member (DECISIONS 339).
func (g *gen) writeOpen(s *enumSpec) {
	v := openView{
		Name: s.name, String: ir.GoString, Wire: ir.GoWire, Known: ir.GoKnown, Code: ir.GoCode,
		Parse: g.names.ParseName(s.name), FromCode: g.names.FromCodeName(s.name), Arg: s.parseArg, Codes: s.codes,
	}
	consts := make([]string, len(s.consts))
	for i, c := range s.consts {
		consts[i] = c.name
		v.Members = append(v.Members, openMember{Const: c.name, Text: strconv.Quote(s.names[i]), Number: s.numbers[i]})
	}
	v.Consts = strings.Join(consts, listSep)
	g.exec(tmplOpenEnum, v)
	g.writeMembers(s)
	if s.codes != "" {
		g.exec(tmplOpenCodes, v)
	}
}
