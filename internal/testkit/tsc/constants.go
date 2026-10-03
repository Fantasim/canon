package tsc

const (
	// maxParentDirs bounds the walk up to tools/tsc, from a package directory or a worktree.
	maxParentDirs = 8

	toolsDir     = "tools/tsc"
	modulesDir   = "node_modules"
	typingsDir   = "@types"
	compilerPath = "bin/tsc"
	nodeName     = "node"
	noNodeMsg    = "node not found on PATH"
	oneTscMsg    = "CANON_REQUIRE_TS needs both TypeScript compilers of tools/tsc (typescript5 and typescript-latest)"
	noTscMsg     = "no TypeScript compiler under tools/tsc/node_modules (run npm ci in tools/tsc)"

	flagStrict      = "--strict"
	flagNoUnused    = "--noUnusedLocals"
	flagTarget      = "--target"
	flagModule      = "--module"
	flagResolution  = "--moduleResolution"
	flagTypes       = "--types"
	flagTypeRoots   = "--typeRoots"
	valueTarget     = "ES2020"
	valueModule     = "ES2020"
	valueResolution = "Bundler"
	valueNodeTypes  = "node"
)

// compilers are the TypeScript compilers of tools/tsc: the 5.0 floor and the current release, by their npm alias.
var compilers = []string{"typescript5", "typescript-latest"}
