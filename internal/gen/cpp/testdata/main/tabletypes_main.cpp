// Decodes JSON files through demo.tabletypes' public Shelf::Decode (CODEGEN.md §4.2, §5.13):
// argv[1] is the directory of the files, then one file name per argument. Prints one line per
// file: the shelf's tables with each row's id, retired flag and n, or the decoder's error. A
// nlohmann::json keeps its object keys in byte order, so rows come out in that order here.
#include "out/tabletypes.gen.h"

#include <nlohmann/json.hpp>

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

namespace {

using demo::tabletypes::Shelf;
using demo::tabletypes::Slot;

std::string Rows(const canon::KeyedList<std::string, Slot>& rows) {
    std::string out;
    for (size_t i = 0; i < rows.Len(); ++i) {
        const Slot& s = rows.At(i);
        if (i > 0) out += ',';
        out += s.GetId() + ":" + std::to_string(s.GetN()) + (s.GetRetired() ? "r" : "");
    }
    return out;
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
        const std::optional<Shelf> shelf = Shelf::Decode(doc, error);
        if (!shelf) {
            std::printf("%s: error %s\n", argv[i], error.c_str());
            continue;
        }
        std::printf("%s: slots=%s extra=%s spare=%s\n", argv[i], Rows(shelf->GetSlots()).c_str(), Rows(shelf->GetExtra()).c_str(),
                    shelf->GetSpare() != nullptr ? Rows(*shelf->GetSpare()).c_str() : "none");
    }
    return 0;
}
