// Reads demo.refdep's Reward through Parse (CODEGEN.md §5.6, DECISIONS 350, 362): the branch of its
// param is decided by its event, an opened enum, through a table keyed by the wire. argv[1] is the
// directory, then one file name per argument. Prints one line per file.
#include "out/refdep/refdep.gen.h"

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

extern std::string g_early;  // open_refdep_early.cpp

using demo::refdep::ParamBranch;
using demo::refdep::Reward;

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string dir = argv[1];
    std::printf("early: %s\n", g_early.c_str());
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::string error;
        const std::optional<Reward> r = Reward::Parse(text, error);
        if (!r) {
            std::printf("%s: error %s\n", argv[i], error.c_str());
            continue;
        }
        std::string param = "none";
        if (r->GetParam() != nullptr) {
            const auto& p = *r->GetParam();
            param = p.GetBranch() == ParamBranch::count ? "count " + std::to_string(*p.AsCount()) : "place " + *p.AsPlace();
        }
        std::printf("%s: event=%s param=%s points=%lld\n", argv[i], std::string(demo::refdep::ToWire(r->GetEventKey())).c_str(), param.c_str(),
                    static_cast<long long>(r->GetPoints()));
    }
    return 0;
}
