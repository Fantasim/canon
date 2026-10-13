// Reads demo.open's text files through the types-mode Parse and Decode<Fn>File (CODEGEN.md §5.2,
// §5.13; DECISIONS 362, 363): argv[1] is their directory, then one argument per file, "cfg:<file>"
// for a Config, "rates:", "names:", "rows:" and "bystat:" for the four @text decoders. Prints
// "== <argument>", then what was read or the decoder's error.
#include "out/open/open.gen.h"

#include <chrono>
#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <unordered_map>

namespace {

using namespace demo::openenum;

// LookupKey<K> is const K& for an opened enum only (CODEGEN.md §5.2, §7.4): any other key keeps its by-value form.
static_assert(std::is_same_v<canon::LookupKey<Dst>, const Dst&>, "an opened enum key is taken by reference");
static_assert(std::is_same_v<canon::LookupKey<std::chrono::milliseconds>, std::chrono::milliseconds>, "a Duration key is taken by value");
static_assert(std::is_same_v<canon::LookupKey<std::string>, std::string_view>, "a string key is a view");
static_assert(std::is_same_v<canon::LookupKey<int64_t>, int64_t>, "an integer key is taken by value");

// A Dst or Stat as "wire/name/known|unknown".
template <class E>
std::string Show(const E& v) {
    return std::string(ToWire(v)) + "/" + std::string(ToName(v)) + "/" + (IsKnown(v) ? "known" : "unknown");
}

std::string Join(const std::vector<Dst>& v) {
    std::string out;
    for (const Dst& d : v) out += (out.empty() ? "" : ",") + Show(d);
    return out;
}

std::string Code(const Stat& s) {
    const auto code = ToCode(s);
    return code ? std::to_string(*code) : "none";
}

void Config_(const std::string& text) {
    std::string error;
    const std::optional<Config> c = Config::Parse(text, error);
    if (!c) {
        std::printf("error %s\n", error.c_str());
        return;
    }
    std::printf("primary=%s equal_hp=%d less_mp=%d\n", Show(c->GetPrimary()).c_str(), c->GetPrimary() == Dst::hp, Dst::hp < Dst::mp);
    std::printf("extra=%s\n", c->GetExtra() ? Show(*c->GetExtra()).c_str() : "none");
    std::printf("list=%s\n", Join(c->GetList()).c_str());
    std::unordered_map<Dst, int> counts;
    for (const Dst& d : c->GetList()) ++counts[d];
    std::printf("counts hp=%d unknown=%d size=%zu\n", counts.at(Dst::hp), static_cast<int>(counts.count(Dst("DST_NEW"))), counts.size());
    std::printf("stat=%s code=%s\n", Show(c->GetStat()).c_str(), Code(c->GetStat()).c_str());
    std::printf("rank=%s code=%d\n", Show(c->GetRank()).c_str(), static_cast<int>(*ToCode(c->GetRank())));
    for (const Entry& e : c->GetEntries()) std::printf("entry %s n=%lld\n", Show(e.GetDst()).c_str(), static_cast<long long>(e.GetN()));
    const Entry* hp = c->GetEntries().Find(Dst::hp);
    const Entry* unknown = c->GetEntries().Find(Dst("DST_NEW"));
    std::printf("find hp=%lld unknown=%s\n", hp ? static_cast<long long>(hp->GetN()) : -1LL, unknown ? "found" : "absent");
    std::printf("mode=%s\n", c->GetMode().c_str());
    std::string flags;
    for (const Stat& s : c->GetFlags()) flags += (flags.empty() ? "" : ",") + std::string(ToName(s));
    std::printf("flags=%s\n", flags.c_str());
    std::printf("shadeOn=%s branch=%d\n", Show(c->GetShadeOn()).c_str(), static_cast<int>(c->GetShade().GetBranch()));
}

void Rates(const std::string& text) {
    std::string error;
    const auto m = DecodeRatesFile(text, error);
    if (!m) {
        std::printf("error %s\n", error.c_str());
        return;
    }
    for (const auto& kv : *m) std::printf("%s=%lld\n", Show(kv.first).c_str(), static_cast<long long>(kv.second));
    const int64_t* found = m->Find(Dst::mp);
    std::printf("find mp=%lld\n", found ? static_cast<long long>(*found) : -1LL);
}

void Names(const std::string& text) {
    std::string error;
    const auto v = DecodeNamesFile(text, error);
    std::printf(v ? "names %s\n" : "error %s\n", v ? Join(*v).c_str() : error.c_str());
}

void Rows(const std::string& text) {
    std::string error;
    const auto v = DecodeRowsFile(text, error);
    if (!v) {
        std::printf("error %s\n", error.c_str());
        return;
    }
    for (const Entry& e : *v) std::printf("row %s n=%lld\n", Show(e.GetDst()).c_str(), static_cast<long long>(e.GetN()));
    std::printf("find new=%s\n", v->Find(Dst("DST_NEW")) ? "found" : "absent");
}

void ByStat(const std::string& text) {
    std::string error;
    const auto m = DecodeByStatFile(text, error);
    if (!m) {
        std::printf("error %s\n", error.c_str());
        return;
    }
    for (const auto& kv : *m) std::printf("%s -> %s n=%lld\n", Show(kv.first).c_str(), Show(kv.second.GetDst()).c_str(), static_cast<long long>(kv.second.GetN()));
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        const std::string arg = argv[i];
        const size_t colon = arg.find(':');
        const std::string kind = arg.substr(0, colon);
        std::ifstream in(dir + "/" + arg.substr(colon + 1), std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::printf("== %s\n", arg.c_str());
        if (kind == "cfg") Config_(text);
        if (kind == "rates") Rates(text);
        if (kind == "names") Names(text);
        if (kind == "rows") Rows(text);
        if (kind == "bystat") ByStat(text);
    }
    return 0;
}
