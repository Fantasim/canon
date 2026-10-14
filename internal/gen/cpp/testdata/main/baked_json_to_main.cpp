// Writing an id through nlohmann is deleted (CODEGEN.md §5.3, DECISIONS 379).
#include <nlohmann/json.hpp>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    nlohmann::json j = b::StatusId::open;
    return j.is_null() ? 1 : 0;
}
