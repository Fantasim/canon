// Decodes JSON files through demo.ordered's public Shelf::Decode, twice (CODEGEN.md §5.13, DECISIONS
// 437): argv[1] is the directory of the files, then one file name per argument. Prints two lines per
// file, the shelf's maps and nested table or the error: first of Decode of an nlohmann::json, whose
// objects come in key byte order, then (marked "ordered") of Decode of an nlohmann::ordered_json
// parsed from the same text, whose objects keep the file's order.
#include "out/ordered.gen.h"

#include <nlohmann/json.hpp>

#include <cstdint>
#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <vector>

namespace {

using demo::ordered::Rack;
using demo::ordered::Shelf;

std::string Entries(const canon::FlatMap<std::string, int64_t>& m) {
    std::string out;
    for (const auto& e : m) {
        if (!out.empty()) out += ',';
        out += e.first + ":" + std::to_string(e.second);
    }
    return out;
}

std::string Deep(const canon::FlatMap<std::string, canon::FlatMap<std::string, int64_t>>& m) {
    std::string out;
    for (const auto& e : m) {
        if (!out.empty()) out += ';';
        out += e.first + "[" + Entries(e.second) + "]";
    }
    return out;
}

std::string Slots(const canon::KeyedList<std::string, demo::ordered::Slot>& rows) {
    std::string out;
    for (size_t i = 0; i < rows.Len(); ++i) {
        if (i > 0) out += ',';
        out += rows.At(i).GetId() + ":" + std::to_string(rows.At(i).GetN());
    }
    return out;
}

std::string Racks(const std::vector<Rack>& racks) {
    std::string out;
    for (size_t i = 0; i < racks.size(); ++i) {
        if (i > 0) out += ';';
        out += "[" + Entries(racks[i].GetTags()) + "]";
    }
    return out;
}

void Print(const char* label, const char* file, const std::optional<Shelf>& shelf, const std::string& error) {
    if (!shelf) {
        std::printf("%s%s: error %s\n", label, file, error.c_str());
        return;
    }
    std::printf("%s%s: power=%s deep=%s slots=%s racks=%s\n", label, file, Entries(shelf->GetPower()).c_str(), Deep(shelf->GetDeep()).c_str(),
                Slots(shelf->GetSlots()).c_str(), Racks(shelf->GetRacks()).c_str());
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        const nlohmann::json doc = nlohmann::json::parse(text, nullptr, /*allow_exceptions=*/false);
        std::string error;
        Print("", argv[i], Shelf::Decode(doc, error), error);
        const nlohmann::ordered_json ordered = nlohmann::ordered_json::parse(text, nullptr, /*allow_exceptions=*/false);
        std::string orderedError;
        Print("ordered ", argv[i], Shelf::Decode(ordered, orderedError), orderedError);
    }
    return 0;
}
