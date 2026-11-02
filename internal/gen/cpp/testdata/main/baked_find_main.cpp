// The baked-find fixture (DECISIONS 432): every container of a baked emit finds each of its keys at
// the address At(i) gives and nothing for a key it lacks, whatever the key's kind and the order of
// the rows; FindBy over a UInt16 and a String field, and a container of no row, do the same.
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <string>
#include <string_view>

#include "finder.gen.h"

namespace f = demo::finder;

namespace {

int hits = 0;
int misses = 0;
int failures = 0;

void Expect(bool ok, const char* what) {
    if (!ok) {
        ++failures;
        std::printf("FAIL %s\n", what);
    }
}

// Every row is found at its own address by its key.
template <class C, class KeyOf>
void FindsEach(const C& c, const char* what, KeyOf keyOf) {
    for (size_t i = 0; i < c.Len(); ++i) {
        ++hits;
        Expect(c.Find(keyOf(c.At(i))) == &c.At(i), what);
    }
}

template <class C, class K>
void Lacks(const C& c, const char* what, K key) {
    ++misses;
    Expect(c.Find(key) == nullptr, what);
}

std::string_view Bytes(const char* s, size_t n) { return std::string_view(s, n); }

}  // namespace

int main() {
    const f::Names& names = f::GetNames();
    FindsEach(names, "names", [](const f::Entry& e) { return std::string_view(e.GetName()); });
    for (std::string_view k : {Bytes("", 0), Bytes("Y", 1), Bytes("[", 1), Bytes("`", 1), Bytes("aa", 2), Bytes("abc", 3),
                               Bytes("pr", 2), Bytes("pref", 4), Bytes("prefi", 5), Bytes("prefixx", 7), Bytes("a\0c", 3),
                               Bytes("a\0", 2), Bytes("\0", 1), Bytes("\xc3\xa8", 2), Bytes("\xc3\xa9\xc3\xa9", 4), Bytes("\xff", 1)}) {
        Lacks(names, "names", k);
    }
    Expect(names.Find(Bytes("a\0b", 3)) != nullptr && names.Find(Bytes("a\0b", 3))->GetCode() == 7, "nul key");
    for (size_t i = 0; i < names.Len(); ++i) {
        ++hits;
        Expect(names.FindByCode(names.At(i).GetCode()) == &names.At(i), "byCode");
        ++hits;
        Expect(names.FindByAlias(names.At(i).GetAlias()) == &names.At(i), "byAlias");
    }
    for (uint16_t code : {uint16_t{1}, uint16_t{8}, uint16_t{299}, uint16_t{901}, uint16_t{65534}}) {
        ++misses;
        Expect(names.FindByCode(code) == nullptr, "byCode miss");
    }
    for (std::string_view alias : {Bytes("", 0), Bytes("x", 1), Bytes("x1 ", 3), Bytes("\xc3\xa9", 2), Bytes("pq", 2), Bytes("zz", 2)}) {
        ++misses;
        Expect(names.FindByAlias(alias) == nullptr, "byAlias miss");
    }

    const f::Numbers& numbers = f::GetNumbers();
    FindsEach(numbers, "numbers", [](const f::Number& n) { return n.GetN(); });
    for (int64_t k : {int64_t{1}, int64_t{-1}, int64_t{5}, int64_t{-4}, INT64_MIN + 1, INT64_MAX - 1, (int64_t{1} << 62) + 1,
                      -(int64_t{1} << 62) + 1, (int64_t{1} << 62) - 1}) {
        Lacks(numbers, "numbers", k);
    }

    FindsEach(f::GetTones(), "tones", [](const f::ToneRow& r) { return r.GetTone(); });
    for (int k : {3, 7, 255}) Lacks(f::GetTones(), "tones", static_cast<f::Tone>(k));

    FindsEach(f::GetRanks(), "ranks", [](const f::RankRow& r) { return r.GetRank(); });
    for (int k : {0, 2, 3, 5, 8, 10, 255}) Lacks(f::GetRanks(), "ranks", static_cast<f::Rank>(k));

    const f::Kinds& kinds = f::GetKinds();
    FindsEach(kinds, "kinds", [](const f::Kind& k) { return std::string(f::ToWire(k.GetId())); });
    for (size_t i = 0; i < kinds.Len(); ++i) Expect(&kinds.Get(kinds.At(i).GetId()) == &kinds.At(i), "kinds get");
    for (std::string_view k : {Bytes("", 0), Bytes("Zeta", 4), Bytes("zet", 3), Bytes("zeta ", 5), Bytes("zz", 2), Bytes("a", 1)}) {
        Lacks(kinds, "kinds", k);
    }
    Expect(kinds.Find("alpha") == &kinds.At(1) && kinds.At(1).GetRetired(), "retired kind");

    const f::ByKind& byKind = f::GetByKind();
    FindsEach(byKind, "byKind", [](const f::KindRow& r) { return r.GetKindKey(); });
    Lacks(byKind, "byKind", f::KindId::alpha);
    Lacks(byKind, "byKind", static_cast<f::KindId>(9));

    FindsEach(f::GetLinks(), "links", [](const f::LinkRow& r) { return std::string_view(r.GetLinkKey()); });
    for (std::string_view k : {Bytes("", 0), Bytes("b", 1), Bytes("z", 1), Bytes("_", 1), Bytes("aa", 2), Bytes("Z ", 2)}) {
        Lacks(f::GetLinks(), "links", k);
    }

    Expect(f::GetEmpties().Len() == 0, "empties");
    Lacks(f::GetEmpties(), "empties", Bytes("", 0));
    Lacks(f::GetEmpties(), "empties", Bytes("a", 1));

    std::printf("names=%zu numbers=%zu tones=%zu ranks=%zu kinds=%zu byKind=%zu links=%zu empties=%zu\n", names.Len(), numbers.Len(),
                f::GetTones().Len(), f::GetRanks().Len(), kinds.Len(), byKind.Len(), f::GetLinks().Len(), f::GetEmpties().Len());
    std::printf("hits=%d misses=%d failures=%d\n", hits, misses, failures);
    return failures == 0 ? 0 : 1;
}
