// Reads a demo.user Holder through Parse (CODEGEN.md §2.8, §5.2; DECISIONS 362): its fields hold the
// opened enums of demo.open, which it names from its own namespace. argv[1] is the directory, then
// one file name per argument. Prints one line per file.
#include "out/user/user.gen.h"

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::string error;
        const std::optional<demo::user::Holder> h = demo::user::Holder::Parse(text, error);
        if (!h) {
            std::printf("%s: error %s\n", argv[i], error.c_str());
            continue;
        }
        std::string dsts;
        for (const demo::openenum::Dst& d : h->GetDsts()) dsts += std::string(demo::openenum::ToWire(d)) + (demo::openenum::IsKnown(d) ? "+" : "?") + " ";
        std::printf("%s: dst=%s known=%d dsts=%sstat=%s fixed=%s\n", argv[i], std::string(demo::openenum::ToWire(h->GetDst())).c_str(),
                    demo::openenum::IsKnown(h->GetDst()), dsts.c_str(), std::string(demo::openenum::ToWire(h->GetStat())).c_str(),
                    std::string(ToWire(h->GetFixed())).c_str());
    }
    return 0;
}
