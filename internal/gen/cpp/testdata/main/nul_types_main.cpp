// The types-nul fixture (CODEGEN.md §3.4, §5.13, DECISIONS 392): a case wire holding a NUL is
// compared whole: the full wire decodes as its case, the wire cut at the NUL is an unknown case.
#include <cstdio>
#include <string>

#include <nlohmann/json.hpp>

#include "nul.gen.h"

namespace n = demo::nul;

static void decode(const char* what, const nlohmann::json& j) {
    std::string error;
    const auto h = n::Holder::Decode(j, error);
    const n::PrizeCoins* coins = h ? h->GetPrize().AsCoins() : nullptr;
    std::printf("%s: %s amount=%lld\n", what, h ? "ok" : error.c_str(), coins ? static_cast<long long>(coins->GetAmount()) : -1LL);
}

int main() {
    nlohmann::json whole;
    whole["prize"] = {{"kind", std::string("a\0b", 3)}, {"amount", 5}};
    nlohmann::json cut;
    cut["prize"] = {{"kind", "a"}, {"amount", 5}};
    decode("whole", whole);
    decode("cut", cut);
    std::printf("wire=%zu\n", n::ToWire(n::PrizeKind::coins).size());
    return 0;
}
