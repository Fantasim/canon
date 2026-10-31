// Reading an enum without @codes through nlohmann is deleted (CODEGEN.md §5.2, DECISIONS 402).
#include <nlohmann/json.hpp>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    const nlohmann::json j = 1;
    return j.get<b::Tone>() == b::Tone::loud ? 0 : 1;
}
