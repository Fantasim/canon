// Loads e.a's zone, whose e.b Band has a lookup over e.b's empty table (CODEGEN.md §5.10):
// argv[1] holds zone.json. Prints one line per failing check.
#include "e/a/out/a.gen.h"

#include <cstdio>
#include <memory>
#include <string>

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    int failures = 0;
    std::string error;
    std::shared_ptr<const e::a::Zone> zone = e::a::Zone::Load(std::string(argv[1]) + "/zone.json", error);
    if (zone == nullptr || zone->GetBand().GetBase() != 3) {
        std::printf("FAIL Zone::Load: %s\n", error.c_str());
        ++failures;
    }
    std::printf("failures: %d\n", failures);
    return failures;
}
