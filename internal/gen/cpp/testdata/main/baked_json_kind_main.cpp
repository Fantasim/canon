// Writing a variant's kind enum through nlohmann is deleted (CODEGEN.md §5.5, DECISIONS 402).
#include <nlohmann/json.hpp>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    nlohmann::json j = b::RewardKind::coins;
    return j.is_null() ? 1 : 0;
}
