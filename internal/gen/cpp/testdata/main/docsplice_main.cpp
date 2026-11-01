// The docsplice fixture (CODEGEN.md §2.6, CG-06, DECISIONS 423): docs with a line ending in a
// backslash do not swallow the line after them, so every getter and enumerator is still declared, and
// a `/*` or `*/` in such a doc raises no -Wcomment.
#include <cstdio>
#include <string>

#include "docsplice.gen.h"

namespace d = demo::docsplice;

int main() {
    std::string error;
    const auto item = d::Item::Parse(R"({"label": "x", "count": 3, "dir": "src/", "kind": "c"})", error);
    if (!item) {
        std::printf("error %s\n", error.c_str());
        return 1;
    }
    std::printf("label=%s count=%lld dir=%s kind=%s\n", item->GetLabel().c_str(), static_cast<long long>(item->GetCount()),
                item->GetDir().c_str(), std::string(d::ToName(item->GetKind())).c_str());
    std::printf("members=%zu a=%s b=%s d=%s e=%s\n", d::kKindMembers.size(), std::string(d::ToName(d::Kind::a)).c_str(),
                std::string(d::ToName(d::Kind::b)).c_str(), std::string(d::ToName(d::Kind::d)).c_str(),
                std::string(d::ToName(d::Kind::e)).c_str());
    return 0;
}
