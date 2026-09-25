// Package cxx finds the C++ toolchain a generated-code compile test needs (a compiler, g++ or
// clang++, and nlohmann/json) and names the flags and exception modes every such test compiles
// with, so no compile-test package reimplements the search or hardcodes its own copy of them.
// Compilers and Toolchain skip a test whose toolchain is missing, unless CANON_REQUIRE_CXX is
// set, in which case they fail it: the Makefile's check target sets it, so make check cannot
// pass green on a toolchain it never exercised.
package cxx
