package cxx

import "time"

const (
	// maxParentDirs bounds the walk up to Source/External, from a checkout or a .claude/worktrees/ tree.
	maxParentDirs = 8
	// Timeout bounds one compile or one run in a caller's own loop.
	Timeout = 5 * time.Minute
)

// compilerNames are the C++ compilers Toolchain looks for, in order.
var compilerNames = []string{"g++", "clang++"}

// Flags are the flags every compile uses (CODEGEN.md §9, CONFORMANCE.md §5).
var Flags = []string{"-std=c++17", "-Wall", "-Wextra", "-Wpedantic", "-Werror", "-ffp-contract=off", "-O1"}

// Modes are the extra flags a compile-and-run loop uses, once per entry: plain, then no exceptions.
var Modes = [][]string{nil, {"-fno-exceptions"}}
