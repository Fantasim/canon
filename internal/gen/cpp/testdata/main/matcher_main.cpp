// Runs MatchPattern (CODEGEN.md §7.7) directly on raw bytes. stdin: "<patterns> <texts>", then one
// kPattern table per pattern (its length, then its numbers), then each text in hex ("-" when empty).
// stdout: one line per pattern, a '1' or '0' per text.
#include <cstdint>
#include <cstdio>
#include <iostream>
#include <string>
#include <string_view>
#include <vector>

namespace {
#include "MatchPattern.txt"
}  // namespace

int main() {
    size_t patterns = 0, texts = 0;
    if (!(std::cin >> patterns >> texts)) return 100;
    std::vector<std::vector<uint32_t>> tables(patterns);
    for (auto& t : tables) {
        size_t n = 0;
        std::cin >> n;
        t.resize(n);
        for (auto& v : t) std::cin >> v;
    }
    std::vector<std::string> values(texts);
    for (auto& v : values) {
        std::string hex;
        std::cin >> hex;
        if (hex == "-") continue;
        for (size_t i = 0; i + 1 < hex.size(); i += 2) v.push_back(static_cast<char>(std::stoi(hex.substr(i, 2), nullptr, 16)));
    }
    std::string row(texts, '0');
    for (const auto& t : tables) {
        for (size_t i = 0; i < texts; ++i) row[i] = MatchPattern(t.data(), values[i]) ? '1' : '0';
        std::printf("%s\n", row.c_str());
    }
    return 0;
}
