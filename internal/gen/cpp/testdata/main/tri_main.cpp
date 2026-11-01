// The baked-tri fixture (CODEGEN.md §2.6, DECISIONS 423): a string holding `??=`, `??/` and `???`
// compiles under -Werror (no trigraph) and keeps its bytes on every path: a field, short and long lists,
// map keys, constants, a constexpr cell. The driver writes the text with `\?` for the same reason.
#include <cstdio>
#include <string>
#include <string_view>

#include "tri.gen.h"

namespace n = demo::tri;

// show prints whether s is the text.
static void show(const char* what, std::string_view s) {
    std::printf("%s=%d\n", what, s == "a?\?=b ?\?/ c ?\?\?" ? 1 : 0);
}

int main() {
    show("field", n::GetNotes().Get(n::NoteId::n0).GetText());
    show("short", n::GetShort().front());
    show("long", n::GetLong().front());
    show("shortKey", n::GetShortMap().At(0).first);
    show("longKey", n::GetLongMap().At(0).first);
    const int64_t* found = n::GetLongMap().Find(std::string("a?\?=b ?\?/ c ?\?\?"));
    std::printf("find=%lld\n", found ? static_cast<long long>(*found) : -1LL);
    show("const", n::NUL_TEXT);
    show("constShort", n::NUL_SHORT.front());
    show("constLong", n::NUL_LONG.front());
    show("cell", n::SideText(n::Side::left));
    return 0;
}
