package starblock

// Case is a doc text of several lines and the block, unindented, both targets write for it.
type Case struct {
	Name  string
	Doc   string
	Block string
}

// Cases are the doc texts whose block is the same in C++ and TypeScript: no `/*` (C++ splits it, TypeScript does not), no `??/` (C++'s `doc` writes it `?\?/` before the block), at least two lines (TypeScript writes one line as `/** text */`).
func Cases() []Case {
	return []Case{
		{Name: "lines", Doc: "first\nsecond", Block: "/**\n * first\n * second\n */\n"},
		{Name: "empty line", Doc: "first\n\nlast", Block: "/**\n * first\n *\n * last\n */\n"},
		{Name: "comment end", Doc: "ends */ here\nnext", Block: "/**\n * ends *\\/ here\n * next\n */\n"},
		{Name: "backslash", Doc: "joins\\\nnext", Block: "/**\n * joins\\\n * next\n */\n"},
	}
}
