// Loads each argument, a hits.json path, with the generated C++ demo.defs loader and prints one
// line per argument: the load error, or every row's define refs as key=value (CODEGEN.md §5.8).
// The C++ twin of defines_main.go (TestDefineParity).
#include "defs.gen.h"

#include <cstdio>
#include <string>
#include <vector>

namespace {

// Keys and values as "k=v" joined by ',', matching defines_main.go's pairs.
std::string Pairs(const std::vector<std::string>& keys, const std::vector<int64_t>& values) {
    std::string out = "[";
    for (size_t i = 0; i < keys.size(); ++i) {
        if (i > 0) out += ",";
        out += keys[i] + "=" + std::to_string(values[i]);
    }
    return out + "]";
}

std::string Row(const demo::defs::Hit& h) {
    std::string alt = "none";
    if (const std::string* k = h.GetAltKey()) alt = *k + "=" + std::to_string(*h.GetAltValue());
    std::string maybe = "none";
    if (const auto* k = h.GetMaybeKeys()) maybe = Pairs(*k, *h.GetMaybeValues());
    std::string bonuses;
    for (const auto& b : h.GetBonuses()) {
        bonuses += (bonuses.empty() ? "" : ",") + b.GetKindKey() + "=" + std::to_string(b.GetKindValue()) + ":" + std::to_string(b.GetAmount());
    }
    return h.GetId() + " monster=" + h.GetMonsterKey() + "=" + std::to_string(h.GetMonsterValue()) + " alt=" + alt +
           " all=" + Pairs(h.GetAllKeys(), h.GetAllValues()) + " maybe=" + maybe + " deep=" + h.GetDeepKey() + "=" +
           std::to_string(h.GetDeepValue()) + " bonuses=[" + bonuses + "]";
}

}  // namespace

int main(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        std::string error;
        auto hits = demo::defs::Hits::Load(argv[i], error);
        if (hits == nullptr) {
            std::printf("error %s\n", error.c_str());
            continue;
        }
        std::string line;
        for (const auto& h : hits->All()) line += (line.empty() ? "" : " | ") + Row(h);
        std::printf("%s\n", line.c_str());
    }
    return 0;
}
