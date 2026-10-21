// A union over an imported 8-member enum (DECISIONS 392): checked through KindFromWire where the
// header includes the enum's package, by its wires where it does not; both accept and refuse alike.
#include <cstdio>
#include <string>

#include <nlohmann/json.hpp>

#include "demo/bare/out/bare.gen.h"
#include "demo/tagged/out/tagged.gen.h"

template <typename T>
static void decode(const char* what, const char* kind, bool other) {
    nlohmann::json j;
    j["kind"] = kind;
    if (other) j["other"] = "k0";
    std::string error;
    const auto p = T::Decode(j, error);
    std::printf("%s %s: %s\n", what, kind, p ? "ok" : error.c_str());
}

int main() {
    for (const char* k : {"k7", "Title", "k9"}) decode<demo::tagged::Pick>("tagged", k, true);
    for (const char* k : {"k7", "Title", "k9"}) decode<demo::bare::Pick>("bare", k, false);
    return 0;
}
