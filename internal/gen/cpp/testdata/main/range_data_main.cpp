// Loads each argument file "<name>.json" of argv[1] with the generated demo.items loader (CODEGEN.md §5.15).
// Prints "== <name>" and "ok <rows>" or "error " and the loader's message.
#include "items.gen.h"

#include <cstdio>
#include <memory>
#include <string>

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::string error;
        std::printf("== %s\n", argv[i]);
        const auto items = demo::items::Items::Load(dir + "/" + argv[i] + ".json", error);
        if (items) {
            std::printf("ok %zu\n", items->Len());
        } else {
            std::printf("error %s\n", error.c_str());
        }
    }
    return 0;
}
