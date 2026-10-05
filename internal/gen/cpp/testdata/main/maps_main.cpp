// Loads demo.maps' boards through the data loader (CODEGEN.md §5.9, WIRE.md §5.8, DECISIONS 312):
// argv[1] is the directory of the data files, then one file name per argument. Prints one line per
// file: each board's maps in file order, the slots' refs resolved to their boards, or the
// loader's error without the directory.
#include "maps.gen.h"

#include <cstdio>
#include <string>

namespace {

using demo::maps::Board;

std::string Power(const Board& b) {
    std::string out;
    for (const auto& [element, n] : b.GetPower()) {
        if (!out.empty()) out += ',';
        out += std::string(demo::maps::ToWire(element)) + "=" + std::to_string(n);
    }
    return out;
}

std::string Names(const Board& b) {
    std::string out;
    for (const auto& [key, name] : b.GetNames()) {
        if (!out.empty()) out += ',';
        out += key + "=" + name;
    }
    return out;
}

std::string Levels(const Board& b) {
    std::string out;
    for (const auto& [level, name] : b.GetLevels()) {
        if (!out.empty()) out += ',';
        out += std::to_string(level) + "=" + name;
    }
    return out;
}

std::string Groups(const Board& b) {
    std::string out;
    for (const auto& [key, members] : b.GetGroups()) {
        if (!out.empty()) out += ',';
        out += key + "=[";
        for (size_t i = 0; i < members.size(); ++i) out += (i > 0 ? "," : "") + std::to_string(members[i]);
        out += "]";
    }
    return out;
}

// "name:n->board": the slot's count and the board its ref resolved to.
std::string Slots(const Board& b) {
    std::string out;
    for (const auto& [key, slot] : b.GetSlots()) {
        if (!out.empty()) out += ',';
        out += key + ":" + std::to_string(slot.GetN()) + "->" + slot.GetHome().GetId();
    }
    return out;
}

std::string Extra(const Board& b) {
    if (b.GetExtra() == nullptr) return "none";
    std::string out;
    for (const auto& [key, n] : *b.GetExtra()) {
        if (!out.empty()) out += ',';
        out += key + "=" + std::to_string(n);
    }
    return out;
}

// Every map of integer or String keys and scalar values, "key=value" in file order.
template <typename Map, typename Key>
std::string Pairs(const Map& m, Key text) {
    std::string out;
    for (const auto& [key, v] : m) {
        if (!out.empty()) out += ',';
        out += text(key) + "=" + v;
    }
    return out;
}

template <typename Map, typename Key>
std::string Counts(const Map& m, Key text) {
    std::string out;
    for (const auto& [key, n] : m) {
        if (!out.empty()) out += ',';
        out += text(key) + "=" + std::to_string(n);
    }
    return out;
}

// "key:n->board" for a map of slots.
template <typename Map, typename Key>
std::string SlotsOf(const Map& m, Key text) {
    std::string out;
    for (const auto& [key, slot] : m) {
        if (!out.empty()) out += ',';
        out += text(key) + ":" + std::to_string(slot.GetN()) + "->" + slot.GetHome().GetId();
    }
    return out;
}

// "outer=[inner=v,...],...": a map of maps.
std::string Deep(const Board& b) {
    std::string out;
    for (const auto& [key, inner] : b.GetDeep()) {
        if (!out.empty()) out += ',';
        out += key + "=[" + Pairs(inner, [](int64_t k) { return std::to_string(k); }) + "]";
    }
    return out;
}

std::string More(const Board& b) {
    const auto num = [](auto k) { return std::to_string(static_cast<int64_t>(k)); };
    const auto text = [](const std::string& k) { return k; };
    std::string out = " ranks=" + Counts(b.GetRanks(), num) + " tones=" + Counts(b.GetTones(), text) + " byLit=" + SlotsOf(b.GetByLit(), text) + " moods=" + Pairs(b.GetMoods(), text);
    out += " small=" + Pairs(b.GetSmall(), num) + " byBoard=" + Counts(b.GetByBoard(), text) + " byNode=" + Pairs(b.GetByNode(), num);
    out += " deep=" + Deep(b) + " rankSlots=" + SlotsOf(b.GetRankSlots(), num) + " elemSlots=" + SlotsOf(b.GetElemSlots(), [](demo::maps::Element k) { return std::string(demo::maps::ToWire(k)); });
    out += " totals=" + Counts(b.Totals(), text) + " byBoardTotals=" + Counts(b.ByBoardTotals(), text);
    out += " perTone=plain:" + Counts(b.PerTone(demo::maps::Tone::plain), text) + ";series-1:" + Counts(b.PerTone(demo::maps::Tone::series_1), text);
    return out;
}

std::string Dump(const demo::maps::Boards& rows) {
    std::string out;
    for (const Board& b : rows.All()) {
        if (!out.empty()) out += ' ';
        out += b.GetId() + "[power=" + Power(b) + " names=" + Names(b) + " levels=" + Levels(b) + " groups=" + Groups(b);
        out += " slots=" + Slots(b) + " extra=" + Extra(b) + More(b) + "]";
    }
    return out;
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::string error;
        const auto boards = demo::maps::Boards::Load(dir + "/" + argv[i], error);
        if (boards == nullptr) {
            std::printf("%s: error %s\n", argv[i], error.substr(dir.size() + 1).c_str());
        } else {
            std::printf("%s: %s\n", argv[i], Dump(*boards).c_str());
        }
    }
    return 0;
}
