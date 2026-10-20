// Drives demo.uses (DECISIONS 383, CODEGEN.md §5.10): its lookups over demo.codes' @codes enum
// find each cell through ::demo::codes::detail::ElementOrdinal, constexpr and stored.
#include <cstdio>
#include <string>

#include "demo/uses/out/uses.gen.h"

namespace c = demo::codes;
namespace u = demo::uses;

static_assert(u::WeightOf(c::Element::EARTH) == 9, "a constexpr lookup over an imported @codes enum");
static_assert(u::WeightOf(c::Element::WATER) == 1, "WATER is second, whatever its code");

int main() {
    for (const c::Element e : c::kElementMembers) {
        const std::string_view name = ToName(e);
        std::printf("%.*s %lld [", static_cast<int>(name.size()), name.data(), static_cast<long long>(u::WeightOf(e)));
        const char* sep = "";
        for (const std::string& label : u::LabelsOf(e)) {
            std::printf("%s%s", sep, label.c_str());
            sep = " ";
        }
        std::printf("]\n");
    }
    return 0;
}
