// The Float32 refusals the parity drivers cannot reach (log-2026-09-24 A1 C++ Float32): a
// midpoint read with no token on record fails loudly, never falls back to nlohmann's double, and
// a Decoder cannot be given a temporary token table. Prints one line per read: the Float32's
// bits, or the decoder's error.
#include "canon_runtime_json.h"

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <type_traits>

using canon::json::Decoder;
using canon::json::Float32Tokens;
using canon::json::Json;

static_assert(std::is_constructible_v<Decoder, std::string, const Float32Tokens&>, "a table that outlives it");
static_assert(!std::is_constructible_v<Decoder, std::string, Float32Tokens&&>, "no temporary table");
static_assert(!std::is_constructible_v<Decoder, std::string, Float32Tokens>, "no temporary table");

namespace {

// value.w is 1 + 2^-24 + 1e-20: its double is the midpoint 1 + 2^-24, its Float32 1 + 2^-23.
const char* const kText = R"({"$schema": "s", "value": {"w": 1.000000059604644775400625}})";

// Reads `w` as value.w with `dec`: its bits, or the decoder's error.
void Read(Decoder& dec, const Json& w) {
    dec.Push("value");
    float f = 0.0f;
    const bool ok = dec.AsFloat32(w, "w", f);
    dec.Pop();
    uint32_t u = 0;
    std::memcpy(&u, &f, sizeof u);
    if (ok) {
        std::printf("%08x\n", static_cast<unsigned>(u));
    } else {
        std::printf("%s\n", dec.Error().c_str());
    }
}

}  // namespace

int main() {
    Json doc;
    std::string error;
    Float32Tokens tokens;
    if (!canon::json::ParseDataFile("f.json", kText, "s", doc, error, tokens)) return std::printf("%s\n", error.c_str()), 1;
    const Json& parsed = doc;
    Decoder dec("f.json", tokens);
    Read(dec, parsed["value"]["w"]);

    Json bare;
    if (!canon::json::ParseDataFile("g.json", kText, "s", bare, error)) return std::printf("%s\n", error.c_str()), 1;
    const Json& bareParsed = bare;
    Decoder noTable("g.json");
    Read(noTable, bareParsed["value"]["w"]);

    const Json copy = parsed["value"]["w"];
    Decoder other("f.json", tokens);
    Read(other, copy);
    return 0;
}
