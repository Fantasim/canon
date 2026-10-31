// Writing an enum without @codes through nlohmann is deleted (CODEGEN.md §5.2, DECISIONS 402).
#include <nlohmann/json.hpp>

#include "board.gen.h"

namespace b = demo::board;

int main() {
    nlohmann::json j = b::Tone::loud;
    return j.is_null() ? 1 : 0;
}
