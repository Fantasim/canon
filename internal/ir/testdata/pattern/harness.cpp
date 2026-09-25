// Differential harness for ir.CppPattern (EVALUATION.md §11.3; log 2026-09-24 "Pattern
// semantics"). stdin: the input count, one hex-encoded input per line, then one translated
// pattern per line. stdout: per pattern, one '0'/'1' per input (std::regex_search with the
// ECMAScript grammar over the input's bytes), or "E" when std::regex refuses the pattern.
// argv[1], when given, names the global locale set before any regex is built.
#include <cstddef>
#include <iostream>
#include <locale>
#include <regex>
#include <string>
#include <vector>

namespace {
int HexValue(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    return c - 'a' + 10;
}

std::string Unhex(const std::string& hex) {
    std::string out;
    for (std::size_t i = 0; i + 1 < hex.size(); i += 2) {
        out.push_back(static_cast<char>(HexValue(hex[i]) * 16 + HexValue(hex[i + 1])));
    }
    return out;
}
}  // namespace

int main(int argc, char** argv) {
    if (argc > 1) std::locale::global(std::locale(argv[1]));
    std::string line;
    std::getline(std::cin, line);
    std::vector<std::string> inputs(std::stoul(line));
    for (auto& in : inputs) {
        std::getline(std::cin, line);
        in = Unhex(line);
    }
    while (std::getline(std::cin, line)) {
        try {
            std::regex re(line, std::regex::ECMAScript);
            std::string row;
            for (const auto& in : inputs) row.push_back(std::regex_search(in, re) ? '1' : '0');
            std::cout << row << '\n';
        } catch (const std::regex_error&) {
            std::cout << "E\n";
        }
    }
    return 0;
}
