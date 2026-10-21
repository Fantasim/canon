// The baked-nul fixture (CODEGEN.md §3.4, DECISIONS 392): a string holding a NUL keeps its three
// bytes on every path: a field, short and long lists, map keys, constants, a constexpr cell.
#include <cstdio>
#include <string>
#include <string_view>

#include "nul.gen.h"

namespace n = demo::nul;

// show prints what s holds: its length and whether its bytes are 'a', NUL, 'b'.
static void show(const char* what, std::string_view s) {
    const bool whole = s.size() == 3 && s[0] == 'a' && s[1] == '\0' && s[2] == 'b';
    std::printf("%s=%zu:%d\n", what, s.size(), whole ? 1 : 0);
}

int main() {
    show("field", n::GetNotes().Get(n::NoteId::n0).GetText());
    show("short", n::GetShort().front());
    show("long", n::GetLong().front());
    show("shortKey", n::GetShortMap().At(0).first);
    show("longKey", n::GetLongMap().At(0).first);
    const std::string key("a\0b", 3);
    const int64_t* found = n::GetLongMap().Find(key);
    std::printf("find=%lld\n", found ? static_cast<long long>(*found) : -1LL);
    show("const", n::NUL_TEXT);
    show("constShort", n::NUL_SHORT.front());
    show("constLong", n::NUL_LONG.front());
    show("cell", n::SideText(n::Side::left));
    static_assert(n::NUL_TEXT.size() == 3, "a constexpr constant keeps its NUL");
    return 0;
}
