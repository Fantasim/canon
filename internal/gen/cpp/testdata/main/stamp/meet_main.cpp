// Identical copies of both runtime texts from two output directories, a/ and b/, meet in one
// translation unit (CODEGEN.md §7.4, DECISIONS 436); the evaluation-error handler is shared with
// same.cpp (copy b/) and not with other.cpp (a copy of another stamp).
#include "a/canon_runtime.h"
#include "a/canon_runtime_json.h"
#include "b/canon_runtime.h"
#include "b/canon_runtime_json.h"

#include <cstdint>
#include <cstdio>
#include <string>
#include <string_view>

canon::EvalErrorHandler SwapSame(canon::EvalErrorHandler handler);
canon::EvalErrorHandler SwapOther(canon::EvalErrorHandler handler);

namespace {

void Quiet(std::string_view, std::string_view) {}

}  // namespace

int main() {
    const auto map = canon::FlatMap<std::string, int64_t>::FromEntries({{"b", 2}, {"a", 1}});
    const canon::json::Json doc = canon::json::Json::parse(R"({"n": 3})", nullptr, false);
    canon::json::Decoder dec("meet");
    int64_t n = 0;
    const bool read = dec.Int(doc, "n", n);
    std::printf("map %lld, decoded %d %lld\n", static_cast<long long>(*map.Find("a")), read ? 1 : 0,
                static_cast<long long>(n));

    canon::SetEvalErrorHandler(&Quiet);
    const bool same = SwapSame(nullptr) == &Quiet;
    canon::SetEvalErrorHandler(&Quiet);
    const bool other = SwapOther(nullptr) == &Quiet;
    canon::SetEvalErrorHandler(nullptr);
    std::printf("handler shared: same %d, other %d\n", same ? 1 : 0, other ? 1 : 0);
    return 0;
}
