// A decode during static initialisation, from a file linked before refdep.gen.cpp (CODEGEN.md §5.6):
// every branch table is a constant expression, so the lookup needs no other file's initialiser.
#include "out/refdep/refdep.gen.h"

#include <optional>
#include <string>

std::string g_early;

namespace {

const bool early = [] {
    std::string error;
    const std::optional<demo::refdep::Reward> r =
        demo::refdep::Reward::Parse(R"({"event": "e_beat", "param": 5, "points": 1})", error);
    g_early = r ? "ok" : error;
    return true;
}();

}  // namespace
