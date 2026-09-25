// Loads dependent-typed fields through the data loader (CODEGEN.md §5.6, §7.6): argv[1] holds one
// data file per case. Prints one line per file: each branch read, or the loader's error without
// the directory.
#include "demo.gen.h"

#include <cstdio>
#include <memory>
#include <string>

namespace {

std::string Payload(const demo::Payload& p) {
    if (const std::string* s = p.AsFalse()) return "false:" + *s;
    if (const auto k = p.AsTrue()) return "true:" + std::string(demo::ToWire(*k));
    return "?";
}

std::string Multi(const demo::Multi* m) {
    if (m == nullptr) return "none";
    const std::string branch = std::to_string(static_cast<int>(m->GetBranch()));
    if (const std::string* a = m->AsA()) return branch + ":a:" + *a;
    if (const std::string* b = m->AsB()) return branch + ":b:" + *b;
    if (const auto c = m->AsC()) return branch + ":c:" + std::to_string(*c);
    return "?";
}

std::string Deep(const demo::Depth& d) {
    if (const auto a = d.AsA()) return std::string("a:") + (*a ? "true" : "false");
    if (const auto b = d.AsB()) return "b:" + std::to_string(*b);
    return "?";
}

void Show(const std::string& dir, const char* name) {
    std::string error;
    auto e = demo::Event::Load(dir + "/" + name + ".json", error);
    if (e == nullptr) {
        std::printf("%s: error %s\n", name, error.substr(dir.size() + 1).c_str());
        return;
    }
    std::string many;
    for (const demo::Payload& p : e->GetMany()) many += " " + Payload(p);
    std::printf("%s: payload=%s multi=%s deep=%s many=%s\n", name, Payload(e->GetPayload()).c_str(), Multi(e->GetMulti()).c_str(),
                Deep(e->GetDeep()).c_str(), many.c_str());
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    for (const char* name : {"ok1", "ok2", "ok3", "ok4", "neverwritten", "badenum", "badint", "badbranch"}) {
        Show(argv[1], name);
    }
    return 0;
}
