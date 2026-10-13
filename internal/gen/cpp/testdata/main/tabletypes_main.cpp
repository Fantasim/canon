// Decodes JSON files through demo.tabletypes' public Shelf::Decode and Shelf::Parse (CODEGEN.md
// §4.2, §5.13): argv[1] is the directory of the files, then one file name per argument. Prints two
// lines per file: the shelf's tables with each row's id, retired flag and n, or the error, first
// of Decode, then (marked "parse") of Parse. A nlohmann::json keeps its object keys in byte order,
// so Decode's rows come out in that order; Parse reads the text and keeps the file's order.
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

void Print(const char* label, const char* file, const std::optional<Shelf>& shelf, const std::string& error) {
    if (!shelf) {
        std::printf("%s%s: error %s\n", label, file, error.c_str());
        return;
    }
    std::printf("%s%s: slots=%s extra=%s spare=%s\n", label, file, Rows(shelf->GetSlots()).c_str(), Rows(shelf->GetExtra()).c_str(),
                shelf->GetSpare() != nullptr ? Rows(*shelf->GetSpare()).c_str() : "none");
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
        std::string parseError;
        Print("parse ", argv[i], Shelf::Parse(text, parseError), parseError);
    }
    return 0;
}
