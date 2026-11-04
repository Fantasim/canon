// Decodes JSON files through demo.parts' public Row::Parse, a record whose only fields are @json(inline)
// variants (Reward, read at the key "kind", then Cost, read at "cost"), and prints every one of them, or
// the error (CODEGEN.md §2.11, §7.6): argv[1] is the directory of the files, then one
// file name per argument. The same program runs against the package generated whole and cut into
// Decode<N> parts, so a part holding an inline unit is compiled; the outputs must be equal, the first
// failure included.
#include "out/parts.gen.h"

#include <cstdint>
#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

namespace {

using demo::parts::Row;

std::string Reward(const demo::parts::Reward& r) {
    if (const auto* g = r.AsGem()) return "gem" + std::to_string(g->GetN());
    return "none";
}

std::string Cost(const demo::parts::Cost& c) {
    if (const auto* k = c.AsCoin()) return "coin" + std::to_string(k->GetAmount());
    return "free";
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::string error;
        const std::optional<Row> row = Row::Parse(text, error);
        if (!row) {
            std::printf("%s: error %s\n", argv[i], error.c_str());
            continue;
        }
        std::printf("%s:", argv[i]);
        std::printf(" r0=%s", Reward(row->GetR0()).c_str());
        std::printf(" r1=%s", Reward(row->GetR1()).c_str());
        std::printf(" r2=%s", Reward(row->GetR2()).c_str());
        std::printf(" r3=%s", Reward(row->GetR3()).c_str());
        std::printf(" r4=%s", Reward(row->GetR4()).c_str());
        std::printf(" r5=%s", Reward(row->GetR5()).c_str());
        std::printf(" r6=%s", Reward(row->GetR6()).c_str());
        std::printf(" r7=%s", Reward(row->GetR7()).c_str());
        std::printf(" c0=%s", Cost(row->GetC0()).c_str());
        std::printf(" c1=%s", Cost(row->GetC1()).c_str());
        std::printf(" c2=%s", Cost(row->GetC2()).c_str());
        std::printf(" c3=%s", Cost(row->GetC3()).c_str());
        std::printf("\n");
    }
    return 0;
}
