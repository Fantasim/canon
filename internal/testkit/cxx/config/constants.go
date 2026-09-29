package config

// RequireCxxEnv, when set, fails cxx's toolchain gate instead of skipping it.
const RequireCxxEnv = "CANON_REQUIRE_CXX"

// NlohmannIncludeEnv, when set, is the nlohmann/json include directory cxx.Toolchain trusts
// instead of searching Source/External or the usual system install directories: the override
// Windows CI needs, where no such directory exists.
const NlohmannIncludeEnv = "CANON_NLOHMANN_INCLUDE"

// RequireMSVCEnv, when set, fails testkit/golden's MSVC golden-compile gate instead of
// skipping it: only meaningful on Windows, where cl.exe can exist.
const RequireMSVCEnv = "CANON_REQUIRE_MSVC"
