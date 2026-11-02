// detail::DecodeOrdered reads an nlohmann::ordered_json's objects in the order the value holds,
// at every depth, arrays included, and names the first failure under the decoder's name
// (CODEGEN.md §7.5, DECISIONS 437). Prints one line per text: the keys walked, or the error.
#include "a/canon_runtime_json.h"

#include <cstdio>
#include <optional>
#include <string>
#include <vector>

namespace {

using canon::json::Decoder;
using canon::json::Json;

bool Walk(const Json& v, Decoder& dec, std::string& out);

bool WalkInto(const Json& v, const std::string& segment, Decoder& dec, std::string& out) {
    dec.Push(segment);
    const bool ok = Walk(v, dec, out);
    dec.Pop();
    return ok;
}

// The keys of every object under `v`, as the decoder reads them; each object's keys must be
// numbers, objects or arrays of objects.
bool Walk(const Json& v, Decoder& dec, std::string& out) {
    std::vector<canon::json::detail::MapEntry> members;
    if (!canon::json::detail::Members(v, dec, members)) return false;
    for (const auto& m : members) {
        out += *m.key + " ";
        if (m.value->is_object() && !WalkInto(*m.value, *m.key, dec, out)) return false;
        if (!m.value->is_array()) continue;
        for (size_t i = 0; i < m.value->size(); ++i) {
            if (!WalkInto((*m.value)[i], *m.key + "[" + std::to_string(i) + "]", dec, out)) return false;
        }
    }
    return true;
}

}  // namespace

int main() {
    const char* const texts[] = {
        R"({"b": 1, "a": {"d": 1, "c": 2}, "l": [{"z": 1, "y": 2}, {}], "m": {"e": 1, "f": 2}})",
        R"({})",
        R"({"b": {"y": 1, "x": [1]}, "a": 2})",
    };
    for (const char* text : texts) {
        const nlohmann::ordered_json v = nlohmann::ordered_json::parse(text, nullptr, false);
        std::string error;
        const std::optional<std::string> keys =
            canon::json::detail::DecodeOrdered<std::string>(v, "ordered", &Walk, error);
        std::printf("%s\n", keys ? ("keys: " + *keys).c_str() : ("error: " + error).c_str());
    }
    return 0;
}
