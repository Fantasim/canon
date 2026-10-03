// Loads demo.tables' shelves through the data loader (CODEGEN.md §4.2, §5.8; WIRE.md §5.7):
// argv[1] is the directory of the data files, then one file name per argument. Prints one line per
// file: each shelf's nested slots in order, with their retired flags and resolved shelves, or the
// loader's error without the directory.
#include "tables.gen.h"

#include <cstdio>
#include <memory>
#include <string>

namespace {

using demo::tables::Shelf;
using demo::tables::Slot;

// "id:n", an "r" after a retired slot, "@key->shelf" for its ref and the shelf it resolved to.
std::string Slots(const canon::KeyedList<std::string, Slot>& rows) {
    std::string out;
    for (size_t i = 0; i < rows.Len(); ++i) {
        const Slot& s = rows.At(i);
        if (i > 0) out += ',';
        out += s.GetId() + ":" + std::to_string(s.GetN()) + (s.GetRetired() ? "r" : "") + "@" + s.GetHomeKey() + "->" + s.GetHome().GetId();
    }
    return out;
}

std::string Dump(const demo::tables::Shelves& rows) {
    std::string out;
    for (const Shelf& s : rows.All()) {
        if (!out.empty()) out += ' ';
        out += s.GetId() + "[slots=" + Slots(s.GetSlots()) + " spare=";
        out += s.GetSpare() != nullptr ? Slots(*s.GetSpare()) : "none";
        out += "]";
    }
    return out;
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::string error;
        const auto shelves = demo::tables::Shelves::Load(dir + "/" + argv[i], error);
        if (shelves == nullptr) {
            std::printf("%s: error %s\n", argv[i], error.substr(dir.size() + 1).c_str());
        } else {
            std::printf("%s: %s\n", argv[i], Dump(*shelves).c_str());
        }
    }
    return 0;
}
