// Reads demo.rdown's H through rddata's loader and rdtypes' Parse (CODEGEN.md §2.8, §5.6,
// DECISIONS 350, 456): the branch of its param is decided by its event, a ref into a table of
// rdlib, whose ids are strings. argv[1] is the directory holding h.json (the data file), then one
// file name per further argument, parsed as W. Prints one line per file.
#include "rd/data/rddata.gen.h"
#include "rd/typed/rdtypes.gen.h"

#include <cstdio>
#include <fstream>
#include <iterator>
#include <memory>
#include <optional>
#include <string>

namespace {

std::string Show(const demo::rdown::H& h) {
    std::string param = "none";
    if (h.GetParam() != nullptr) {
        const auto& p = *h.GetParam();
        param = p.GetBranch() == demo::rdown::ParamBranch::count ? "count " + std::to_string(*p.AsCount()) : "place " + *p.AsPlace();
    }
    return "ev=" + h.GetEvKey() + " param=" + param;
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string dir = argv[1];
    std::string error;
    const std::shared_ptr<const demo::rdown::H> data = demo::rddata::LoadH(dir + "/h.json", error);
    std::printf("h.json: %s\n", data ? Show(*data).c_str() : ("error " + error).c_str());
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        error.clear();
        const std::optional<demo::rdtypes::W> w = demo::rdtypes::W::Parse(text, error);
        std::printf("%s: %s\n", argv[i], w ? Show(w->GetH()).c_str() : ("error " + error).c_str());
    }
    return 0;
}
