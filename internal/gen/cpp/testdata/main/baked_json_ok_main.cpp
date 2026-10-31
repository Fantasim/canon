// The id is stored through ToWire and read back through StatusIdFromWire; an enum without @codes round-trips through ToWire and ToneFromWire; a @codes enum converts as its code (CODEGEN.md §5.2, §5.3, DECISIONS 379, 402).
#include <nlohmann/json.hpp>
#include <string>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    nlohmann::json j = std::string(b::ToWire(b::StatusId::taken));
    const nlohmann::json plain = std::string(b::ToWire(b::Tone::loud));
    const nlohmann::json coded = b::Element::FIRE;
    const auto id = b::StatusIdFromWire(j.get<std::string>());
    const auto tone = b::ToneFromWire(plain.get<std::string>());
    return id && *id == b::StatusId::taken && tone && *tone == b::Tone::loud && coded.get<b::Element>() == b::Element::FIRE ? 0 : 1;
}
