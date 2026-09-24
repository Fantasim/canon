// Package cxx finds the C++ toolchain a generated-code compile test needs (a compiler, g++ or
// clang++, and nlohmann/json) and names the flags and exception modes every such test compiles
// with, so no compile-test package reimplements the search or hardcodes its own copy of them.
package cxx
