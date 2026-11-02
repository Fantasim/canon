// The baked dependent-map fixture (CODEGEN.md §4.2, §5.6): `{k in rows: T}` is a map of rows' key, read back in every position a map may sit.
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

#include "depmap.gen.h"

namespace d = demo::depmap;

namespace {

template <class K>
void Print(const char* label, const canon::FlatMap<K, int64_t>& m) {
    std::printf("%s=%zu:", label, m.Len());
    for (const auto& e : m) std::printf(" %s=%lld", std::string(d::ToWire(e.first)).c_str(), static_cast<long long>(e.second));
    std::printf("\n");
}

void PrintNest(const char* label, const canon::FlatMap<d::CellId, std::vector<int64_t>>& m) {
    std::printf("%s:", label);
    for (const auto& e : m) {
        std::printf(" %s=", std::string(d::ToWire(e.first)).c_str());
        for (size_t i = 0; i < e.second.size(); ++i) std::printf(i == 0 ? "%lld" : ",%lld", static_cast<long long>(e.second[i]));
        std::printf(";");
    }
    std::printf("\n");
}

}  // namespace

int main() {
    const d::Holder& h = d::GetHolder();
    Print("m", h.GetM());
    std::printf("many=%zu\n", h.GetMany().size());
    for (const auto& m : h.GetMany()) Print("  many", m);
    std::printf("opt=%d absent=%d\n", h.GetOpt() ? 1 : 0, h.GetAbsent() ? 1 : 0);
    Print("opt", *h.GetOpt());
    std::printf("byName=%zu\n", h.GetByName().Len());
    for (const auto& e : h.GetByName()) Print(("  " + e.first).c_str(), e.second);
    std::printf("recs=%zu:", h.GetRecs().Len());
    for (const auto& e : h.GetRecs()) std::printf(" %s=%lld", std::string(d::ToWire(e.first)).c_str(), static_cast<long long>(e.second.GetN()));
    std::printf("\n");
    const d::ShapeWeights* w = h.GetShape().AsWeights();
    Print("shape", w->GetW());
    const int64_t* big = h.GetBig().Find(d::CellId::w7);
    std::printf("big=%zu w7=%lld first=%s last=%s\n", h.GetBig().Len(), big ? static_cast<long long>(*big) : -1LL,
                std::string(d::ToWire(h.GetBig().At(0).first)).c_str(), std::string(d::ToWire(h.GetBig().At(h.GetBig().Len() - 1).first)).c_str());
    std::printf("grid=%zu:", h.GetGrid().size());
    for (const auto& m : h.GetGrid()) {
        std::printf(" [");
        for (const auto& e : m) std::printf("%s%lld", std::string(d::ToWire(e.first)).c_str(), static_cast<long long>(e.second));
        std::printf("]");
    }
    std::printf("\n");
    PrintNest("nest", h.GetNest());
    PrintNest("nestPerFlag(false)", d::NestPerFlag(false));
    PrintNest("nestPerFlag(true)", d::NestPerFlag(true));
    Print("weights", d::GetWeights());
    Print("totals", d::Totals());
    Print("perFlag(false)", d::PerFlag(false));
    Print("perFlag(true)", d::PerFlag(true));
    for (const d::Row& r : d::GetRows().All()) {
        Print(std::string("scores " + std::string(d::ToWire(r.GetId()))).c_str(), r.Scores());
    }
    for (d::CellId id : {d::CellId::w3, d::CellId::w8}) {
        const d::Cell& c = d::GetWide().Get(id);
        const std::string name(d::ToWire(id));
        Print(("cell " + name + " links").c_str(), c.GetLinks());
        Print(("cell " + name + " hops").c_str(), c.Hops());
    }
    const int64_t* found = d::GetWeights().Find(d::RowId::a);
    std::printf("find a=%lld\n", found ? static_cast<long long>(*found) : -1LL);
    return 0;
}
