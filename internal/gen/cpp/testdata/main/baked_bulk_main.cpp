// The baked-bulk fixture (DECISIONS 392): every construct a baked Build reads from arrays, read
// back at run time: a table's columns and per-row blocks, keys, refs, cells and constants.
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <string>

#include "bulk.gen.h"

namespace b = demo::bulk;

int main() {
    for (const b::Item& it : b::GetItems().All()) {
        const b::PrizeCoins* coins = it.GetPrize().AsCoins();
        std::printf("%s retired=%d %s code=%lld tag=%s tier=%s note=%s next=%s coins=%lld twice=%lld bonus=%lld,%lld\n",
                    std::string(b::ToWire(it.GetId())).c_str(), it.GetRetired() ? 1 : 0, it.GetName().c_str(),
                    static_cast<long long>(it.GetCode()), it.GetTags().at(0).c_str(), std::string(b::ToWire(it.GetTier())).c_str(),
                    it.GetNote() ? it.GetNote()->c_str() : "none", it.GetNext().GetName().c_str(),
                    coins ? static_cast<long long>(coins->GetAmount()) : -1LL, static_cast<long long>(it.Twice()),
                    static_cast<long long>(it.Bonus(b::Tier::t0)), static_cast<long long>(it.Bonus(b::Tier::t7)));
    }
    const b::Item* byCode = b::GetItems().FindByCode(105);
    const std::string& odd = b::GetNames()[b::GetNames().size() / 2];
    std::printf("byCode=%s names=%zu %s|%s odd=%zu:%d,%d,%d,%d\n", byCode ? byCode->GetName().c_str() : "none", b::GetNames().size(),
                b::GetNames().front().c_str(), b::GetNames().back().c_str(), odd.size(), odd[1], odd[3], odd[4], odd[6]);
    const int64_t* w = b::GetWeights().Find("w7");
    std::printf("weights=%zu w7=%lld picks=%zu %s %s keys=%s\n", b::GetWeights().Len(), w ? static_cast<long long>(*w) : -1LL,
                b::GetPicks().size(), b::GetPicks().front()->GetName().c_str(), b::GetPicks().back()->GetName().c_str(),
                std::string(b::ToWire(b::GetPicksKeys().front())).c_str());
    const b::Crew* crew = b::GetCrews().Find(int64_t{30});
    const std::string* label = b::LabelOf(b::Tier::t2);
    std::printf("crews=%zu crew30=%s label2=%s label3=%s\n", b::GetCrews().Len(), crew ? crew->GetName().c_str() : "none",
                label ? label->c_str() : "none", b::LabelOf(b::Tier::t3) ? "some" : "none");
    const int64_t* rank = b::TIER_RANKS.Find("t1");
    std::printf("names=%zu %s %s ranks=%zu t1=%lld\n", b::TIER_NAMES.size(), b::TIER_NAMES.front().c_str(), b::TIER_NAMES.back().c_str(),
                b::TIER_RANKS.Len(), rank ? static_cast<long long>(*rank) : -1LL);
    for (const b::Item& it : b::GetItems().All()) {
        std::printf("%s ratio=%g neg=%d weight=%g big=%lld wait=%lld opts=%zu:", std::string(b::ToWire(it.GetId())).c_str(),
                    it.GetRatio(), std::signbit(it.GetRatio()) ? 1 : 0, static_cast<double>(it.GetWeight()),
                    static_cast<long long>(it.GetBig()), it.GetWait() ? static_cast<long long>(it.GetWait()->count()) : -1LL,
                    it.GetOpts().size());
        for (const auto& o : it.GetOpts()) std::printf(" %s", o ? o->c_str() : "none");
        std::printf("\n");
    }
    std::printf("grid=%zu:", b::GetGrid().size());
    for (const auto& row : b::GetGrid()) {
        std::printf(" [");
        for (int64_t x : row) std::printf("%lld", static_cast<long long>(x));
        std::printf("]");
    }
    std::printf("\ngroups=%zu:", b::GetGroups().Len());
    for (const auto& e : b::GetGroups()) {
        std::printf(" %s=", e.first.c_str());
        for (const std::string& x : e.second) std::printf("%s", x.c_str());
    }
    std::printf("\n");
    std::printf("constGroups=%zu:", b::GROUPS.Len());
    for (const auto& e : b::GROUPS) std::printf(" %s=%zu", e.first.c_str(), e.second.size());
    const auto* nul = b::GROUPS.Find("c5");
    std::printf(" c5[0]=%zu:%d\n", nul ? nul->front().size() : 0, nul && nul->front()[1] == '\0' ? 1 : 0);
    std::printf("constGrid=%zu:%zu,%zu,%zu last=%lld\n", b::GRID.size(), b::GRID[0].size(), b::GRID[1].size(), b::GRID[2].size(),
                static_cast<long long>(b::GRID[1].back()));
    return 0;
}
