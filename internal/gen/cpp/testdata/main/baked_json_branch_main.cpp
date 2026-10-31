// Writing a dependent type's branch enum through nlohmann is deleted (CODEGEN.md §5.6, DECISIONS 402).
#include <nlohmann/json.hpp>

#include "demo.gen.h"

int main() {
    nlohmann::json j = demo::PayloadBranch{};
    return j.is_null() ? 1 : 0;
}
