// The bulk fixture (DECISIONS 392): sizes and first, middle and last elements of the data whose
// generated code must not grow with it, read back at run time.
#include <cstdio>
#include <string>

#include "bulk.gen.h"

namespace b = big::bulk;

int main() {
    const b::Header& big = b::GetHeaders().Get(b::HeaderId::big);
    const auto& names = big.GetNames();
    const auto& values = big.GetValues();
    std::printf("names=%zu %s %s %s\n", names.size(), names.front().c_str(), names[names.size() / 2].c_str(),
                names.back().c_str());
    std::printf("values=%zu %lld %lld %lld\n", values.size(), static_cast<long long>(values.front()),
                static_cast<long long>(values[values.size() / 2]), static_cast<long long>(values.back()));
    std::printf("aliases=%zu %s small=%zu empty=%zu file=%s\n", big.GetAliases().size(), big.GetAliases().front().c_str(),
                b::GetHeaders().Get(b::HeaderId::small).GetNames().size(),
                b::GetHeaders().Get(b::HeaderId::empty).GetNames().size(), big.GetFile().c_str());
    const b::Items& items = b::GetItems();
    for (size_t i : {size_t{0}, size_t{1}, items.Len() / 2, items.Len() - 1}) {
        const b::Item& it = items.At(i);
        std::printf("item %s %s level=%lld price=%lld rare=%d cooldown=%lld tags=%s,%s\n", std::string(b::ToWire(it.GetId())).c_str(),
                    it.GetName().c_str(), static_cast<long long>(it.GetLevel()), static_cast<long long>(it.GetPrice()),
                    it.GetRare() ? 1 : 0, static_cast<long long>(it.GetCooldown().count()), it.GetTags()[0].c_str(),
                    it.GetTags()[1].c_str());
        const b::RewardCoins* coins = it.GetReward().AsCoins();
        std::printf("  header=%s key=%s note=%s grade=%s coins=%lld\n", it.GetHeader().GetFile().c_str(),
                    std::string(b::ToWire(it.GetHeaderKey())).c_str(), it.GetNote() ? it.GetNote()->c_str() : "none",
                    std::string(b::ToWire(it.GetGrade())).c_str(), coins ? static_cast<long long>(coins->GetAmount()) : -1LL);
    }
    const b::Item* found = items.Find("II_ITEM_01234");
    std::printf("items=%zu find=%s\n", items.Len(), found ? found->GetName().c_str() : "none");
    const auto& weights = b::GetWeights();
    const int64_t* w = weights.Find("II_ITEM_02999");
    std::printf("weights=%zu first=%s:%lld last=%lld\n", weights.Len(), weights.At(0).first.c_str(),
                static_cast<long long>(weights.At(0).second), w ? static_cast<long long>(*w) : -1LL);
    const auto& groups = b::GetGroups();
    const auto* last = groups.Find("group4999");
    std::printf("groups=%zu first=%s:%zu last=%zu:%s\n", groups.Len(), groups.At(0).first.c_str(), groups.At(0).second.size(),
                last ? last->size() : 0, last && !last->empty() ? last->back().c_str() : "none");
    const auto& grid = b::GetGrid();
    std::printf("grid=%zu:%zu %lld %lld\n", grid.size(), grid.back().size(), static_cast<long long>(grid[1][2]),
                static_cast<long long>(grid.back().back()));
    return 0;
}
