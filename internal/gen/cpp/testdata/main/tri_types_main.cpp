// The types-tri fixture (CODEGEN.md §5.13, DECISIONS 423 pending): a field wire name and a case wire
// holding `??=`, `??/` and `???` compile under -Werror and are compared byte for byte.
#include <cstdio>
#include <string>

#include <nlohmann/json.hpp>

#include "tri.gen.h"

namespace n = demo::tri;

int main() {
    const std::string wire = "a?\?=b ?\?/ c ?\?\?";
    nlohmann::json whole;
    whole[wire] = 7;
    whole["prize"] = {{"kind", wire}, {"amount", 5}};
    std::string error;
    const auto h = n::Holder::Decode(whole, error);
    const n::PrizeCoins* coins = h ? h->GetPrize().AsCoins() : nullptr;
    std::printf("whole: %s count=%lld amount=%lld\n", h ? "ok" : error.c_str(), h ? static_cast<long long>(h->GetCount()) : -1LL,
                coins ? static_cast<long long>(coins->GetAmount()) : -1LL);
    nlohmann::json cut;
    cut[wire] = 7;
    cut["prize"] = {{"kind", "a?\?=b"}, {"amount", 5}};
    const auto c = n::Holder::Decode(cut, error);
    std::printf("cut: %s\n", c ? "ok" : "refused");
    std::printf("wire=%d\n", n::ToWire(n::PrizeKind::coins) == wire ? 1 : 0);
    return 0;
}
