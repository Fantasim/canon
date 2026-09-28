// Sets each argument on both CANON_CHAIN_* variables, runs the generated C++ demo.chain
// LoadInputs and prints one line per argument: the values read, then every failure line joined
// by " | " (CODEGEN.md §5.12, §7.7). The C++ twin of chain_main.go (TestInputPatternChainParity).
#include "chain.gen.h"

#include <cstdio>
#include <cstdlib>
#include <string>

namespace {
// An optional input's value, or none.
std::string Show(const std::string* v) { return v == nullptr ? "none" : *v; }
// err's lines joined by " | ", without the last newline.
std::string Joined(std::string err) {
    if (!err.empty() && err.back() == '\n') err.pop_back();
    std::string out;
    for (char c : err) {
        if (c == '\n') {
            out += " | ";
        } else {
            out.push_back(c);
        }
    }
    return out;
}
}  // namespace

int main(int argc, char** argv) {
    demo::chain::Config cfg;
    for (int i = 1; i < argc; ++i) {
        setenv("CANON_CHAIN_CODE", argv[i], 1);
        setenv("CANON_CHAIN_TAG", argv[i], 1);
        std::string err;
        demo::chain::LoadInputs(err);
        std::printf("code=%s tag=%s err=%s\n", Show(cfg.GetCode()).c_str(), Show(cfg.GetTag()).c_str(), Joined(err).c_str());
    }
    return 0;
}
