// Package cxx finds the C++ toolchain a generated-code compile test needs (a compiler, g++ or
// clang++, and nlohmann/json) and names the flags and exception modes every such test compiles
// with, so no compile-test package reimplements the search or hardcodes its own copy of them.
// Compilers and Toolchain skip a test whose toolchain is missing, unless CANON_REQUIRE_CXX is
// set, in which case they fail it. Wherever clang++ can build against libc++ (libcxx.go), they
// add a second, wrapped row for it beside the platform-default standard library. MSVC's flags
// and argv construction (msvc.go) are exposed for a future Windows caller; cl.exe cannot run
// here, so only their construction is tested.
package cxx
