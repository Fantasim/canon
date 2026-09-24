// Loads each argument, <kind>=<path>, with the generated demo.shop loaders: "shelves" a file
// through Shelves::Load, "reload" a directory through ShopStore::Reload. Prints one line per
// argument: the load error, or "ok".
#include "shop.gen.h"

#include <cstdio>
#include <string>

int main(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        const std::string arg = argv[i];
        const size_t eq = arg.find('=');
        if (eq == std::string::npos) return 100;
        const std::string kind = arg.substr(0, eq), path = arg.substr(eq + 1);
        std::string error;
        if (kind == "shelves") {
            demo::shop::Shelves::Load(path, error);
        } else if (kind == "reload") {
            demo::shop::ShopStore::Reload(path, error);
        } else {
            return 100;
        }
        std::printf("%s\n", error.empty() ? "ok" : error.c_str());
    }
    return 0;
}
