// Drives demo.holders: Item held by two @reload tables, a ref resolving only when every
// holder agrees (CODEGEN.md §5.8, §5.11; log-2026-09-24 "ir name plans + support plan").
#include "holders.gen.h"

#include <cstdio>
#include <string>

using namespace demo::holders;

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    const std::string root = argv[1];
    int failures = 0;
    std::string error;
    if (!HoldersStore::Reload(root + "/good", error)) {
        std::printf("good: %s\n", error.c_str());
        ++failures;
    } else {
        std::shared_ptr<const HoldersSnapshot> snap = HoldersStore::Current();
        const Item* l = snap->GetLeft().Find("L1");
        const Item* r = snap->GetRight().Find("R1");
        if (l == nullptr || r == nullptr || &r->GetPeer() != l || r->GetPeerKey() != "L1") {
            std::printf("good: peer did not resolve\n");
            ++failures;
        }
    }
    error.clear();
    if (HoldersStore::Reload(root + "/bad", error)) {
        std::printf("bad: reload should have failed\n");
        ++failures;
    } else {
        std::printf("bad: %s\n", error.c_str());
    }
    std::printf("failures: %d\n", failures);
    return failures;
}
