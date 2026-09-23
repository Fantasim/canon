package catalog

// Catalog is spec/ERRORS.md read and validated: the input of the generator.
type Catalog struct {
	Codes    []Code // in document order
	ArgTypes []ArgType
	Kinds    []Kind
	Runtime  []RuntimeText
}

// Code is one codes row with its message rows.
type Code struct {
	ID       string
	Severity string
	Package  string
	Owner    string
	Meaning  string
	Messages []Message
	line     int
}

// Message is one message row: Variant is "" for a code with one message.
type Message struct {
	Variant  string
	Args     []Arg
	Template string // escapes kept
	line     int
}

// Arg is one `name:Type` entry of a message's Args.
type Arg struct {
	Name string
	Type string
}

// ArgType is one row of the argument types table: a type and its Go parameter type.
type ArgType struct {
	Name   string
	GoType string
}

// Kind is one row of the kinds table.
type Kind struct {
	Name   string
	Word   string
	UsedBy []string
	line   int
}

// RuntimeText is one (code, text) pair signalled by generated code.
type RuntimeText struct {
	Code string
	Text string
}

// Package is one row of the package table: Dir and Subs are module-relative directories.
type Package struct {
	Name string // "eval", "gen/json", "api", "cmd/canon"
	Dir  string
	Subs []string
}
