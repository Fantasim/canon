// Exercises pattern-checked inputs (EVALUATION.md §11.3; CODEGEN.md §5.12, §7.7). argv[1] holds the
// variables on its first line, then one text per line in hex: each text is set on every variable,
// LoadInputs runs once, and one line prints a mark per variable: 'o' read, 'p' refused for its
// pattern, 'x' refused otherwise.
#include "pat.gen.h"

#include <cstdio>
#include <cstdlib>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>

namespace {
std::string Unhex(const std::string& hex) {
    std::string out;
    for (size_t i = 0; i + 1 < hex.size(); i += 2) out.push_back(static_cast<char>(std::stoi(hex.substr(i, 2), nullptr, 16)));
    return out;
}
// The mark of variable v in LoadInputs' error text err, each line of which names its variable first.
char Mark(const std::string& err, const std::string& v) {
    const std::string lines = "\n" + err;
    const size_t at = lines.find("\n" + v + ": ");
    if (at == std::string::npos) return 'o';
    return lines.compare(at + v.size() + 3, 26, "does not match its pattern") == 0 ? 'p' : 'x';
}
}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    std::ifstream in(argv[1]);
    std::string line;
    std::getline(in, line);
    std::istringstream names(line);
    std::vector<std::string> vars;
    for (std::string v; names >> v;) vars.push_back(v);
    while (std::getline(in, line)) {
        const std::string value = Unhex(line);
        for (const auto& v : vars) setenv(v.c_str(), value.c_str(), 1);
        std::string err;
        pat::LoadInputs(err);
        std::string row;
        for (const auto& v : vars) row.push_back(Mark(err, v));
        std::printf("%s\n", row.c_str());
    }
    return 0;
}
