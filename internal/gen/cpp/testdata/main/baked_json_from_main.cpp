// Reading an id through nlohmann is deleted (CODEGEN.md §5.3, DECISIONS 379).
#include <nlohmann/json.hpp>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    const nlohmann::json j = 1;
    return j.get<b::StatusId>() == b::StatusId::open ? 0 : 1;
}
