package ratchet

const (
	baselineCols = 6
	scanBuf      = 1 << 20
	dirPerm      = 0o755
	filePerm     = 0o644
)

// Baseline TSV column indexes: <rule>\t<file>\t<symbol>\t<detail>\t<count>\t<value>.
const (
	colRule = iota
	colFile
	colSymbol
	colDetail
	colCount
	colValue
)

const header = `# canon audit baseline: the findings this repo already had when the ratchet started.
# <rule>\t<file>\t<symbol>\t<detail>\t<count>\t<value>. Written only by
# 'baseline --init' (once) and '--tighten' (only ever shrinks it).
`
