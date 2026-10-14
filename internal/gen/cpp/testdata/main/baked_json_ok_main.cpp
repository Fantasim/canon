// The id is stored through ToWire and read back through StatusIdFromWire: it compiles (CODEGEN.md §5.3, DECISIONS 379).
#include <nlohmann/json.hpp>
#include <string>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    nlohmann::json j = std::string(b::ToWire(b::StatusId::taken));
    const nlohmann::json plain = b::Tone::loud;  // an enum without the hooks still converts
    const auto id = b::StatusIdFromWire(j.get<std::string>());
    return id && *id == b::StatusId::taken && plain.get<b::Tone>() == b::Tone::loud ? 0 : 1;
}
