package cxx

import "time"

const (
	// maxParentDirs bounds the walk up to Source/External, from a checkout or a .claude/worktrees/ tree.
	maxParentDirs = 8
	// Timeout bounds one compile or one run in a caller's own loop.
	Timeout = 5 * time.Minute

	noCompilerMsg = "no C++ compiler (g++, clang++) on PATH"
	noNlohmannMsg = "nlohmann/json.hpp not found"
	noLibcxxMsg   = "clang++ found but no libc++ install was found for -stdlib=libc++"

	clangxx = "clang++"

	osWindows = "windows"
	osDarwin  = "darwin"
	osLinux   = "linux"

	libcxxFlag       = "-stdlib=libc++"
	wrapperDirName   = "canon-cxx-libcxx"
	wrapperName      = "clang++-libcxx"
	wrapperTmpSuffix = ".tmp"
	wrapperPerm      = 0o755
	wrapperShebang   = "#!/bin/sh\n"

	clExe = "cl.exe"

	msvcIncludeFlag = "/I"
	msvcCompileFlag = "/c"
	msvcObjFlag     = "/Fo"
	msvcExeFlag     = "/Fe"
)

// compilerNames are the C++ compilers Toolchain looks for, in order.
var compilerNames = []string{"g++", clangxx}

// systemIncludeDirs are findNlohmann's system install locations, after Source/External.
var systemIncludeDirs = []string{"/usr/include", "/opt/homebrew/include", "/usr/local/include"}

// libcxxHeaderGlobs are libcxxUsable's libc++-dev include/c++/v1 locations, checked on Linux.
var libcxxHeaderGlobs = []string{
	"/usr/lib/llvm-*/include/c++/v1",
	"/usr/include/c++/v1",
	"/opt/homebrew/opt/llvm/include/c++/v1",
	"/usr/local/opt/llvm/include/c++/v1",
}

// Flags are the flags every compile uses (CODEGEN.md §9, CONFORMANCE.md §5).
var Flags = []string{"-std=c++17", "-Wall", "-Wextra", "-Wpedantic", "-Werror", "-ffp-contract=off", "-O1"}

// Modes are the extra flags a compile-and-run loop uses, once per entry: plain, then no exceptions.
var Modes = [][]string{nil, {"-fno-exceptions"}}

// MSVCFlags are cl.exe's flags for every compile: the MSVC analog of Flags (CODEGEN.md §9).
var MSVCFlags = []string{"/nologo", "/std:c++17", "/W4", "/WX", "/fp:precise"}

// MSVCModes mirrors Modes: exceptions on, then off (no spec recipe pins the second; reported).
var MSVCModes = [][]string{{"/EHsc"}, {"/D_HAS_EXCEPTIONS=0"}}
