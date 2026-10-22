// Drives demo.helpers (DECISIONS 383, CODEGEN.md §5.2): every enum helper the .gen.cpp
// defines, against every member both ways, a value that is no member, an unknown wire and
// code, two members with equal wires, and enums without members.
#include <array>
#include <cstddef>
#include <cstdio>
#include <optional>
#include <string>
#include <string_view>
#include <type_traits>

#include "out/helpers/helpers.gen.h"

namespace h = demo::helpers;

// detail::<E>Ordinal is constexpr: a value's declaration position, the member count for none.
static_assert(h::detail::ElementOrdinal(h::Element::EARTH) == 2, "EARTH is third");
static_assert(h::detail::ElementOrdinal(static_cast<h::Element>(255)) == 3, "255 is no Element");
static_assert(h::detail::DeltaOrdinal(h::Delta::down) == 1, "down is second");
static_assert(h::detail::ExtremeOrdinal(h::Extreme::LOW) == 0, "INT64_MIN is a member");
static_assert(h::detail::ExtremeOrdinal(h::Extreme::HIGH) == 2, "INT64_MAX is a member");
static_assert(h::detail::NoCodeOrdinal(static_cast<h::NoCode>(0)) == 0, "NoCode has no member");

namespace {

void Text(std::string_view s) { std::printf("%.*s", static_cast<int>(s.size()), s.data()); }

template <typename E>
const char* Back(const std::optional<E>& back, const E& m) {
    if (!back) return "none";
    return *back == m ? "self" : "other";
}

// Closed prints each member's name, wire and FromWire(wire), then a value that is no member.
template <typename E, std::size_t N, typename FromWire>
void Closed(const char* label, const std::array<E, N>& members, FromWire from) {
    std::printf("%s:", label);
    for (const E m : members) {
        std::printf(" ");
        Text(ToName(m));
        std::printf("/");
        Text(ToWire(m));
        std::printf("/%s", Back(from(ToWire(m)), m));
    }
    const E none = static_cast<E>(255);
    std::printf(" | none=[");
    Text(ToName(none));
    std::printf("|");
    Text(ToWire(none));
    std::printf("] unknown=%s\n", from("no-such-wire").has_value() ? "found" : "none");
}

// Coded prints each member's code, FromCode(code) and ordinal, then an unknown code's.
template <typename E, std::size_t N, typename FromCode, typename Ordinal>
void Coded(const char* label, const std::array<E, N>& members, FromCode from, Ordinal ordinal, long long unknown) {
    using Code = std::underlying_type_t<E>;
    std::printf("%s:", label);
    for (const E m : members) {
        const Code code = static_cast<Code>(m);
        std::printf(" %lld/%s/%zu", static_cast<long long>(code), Back(from(code), m), ordinal(m));
    }
    const Code bad = static_cast<Code>(unknown);
    std::printf(" | unknown=%s ordinal=%zu\n", from(bad).has_value() ? "found" : "none", ordinal(static_cast<E>(bad)));
}

// Open prints each opened member's name, wire, IsKnown and FromWire(wire), then an unknown wire's.
template <typename E, std::size_t N, typename FromWire>
void Open(const char* label, const std::array<E, N>& members, FromWire from) {
    std::printf("%s:", label);
    for (const E& m : members) {
        std::printf(" ");
        Text(ToName(m));
        std::printf("/");
        Text(ToWire(m));
        std::printf("/%s/%s", IsKnown(m) ? "known" : "unknown", Back(from(ToWire(m)), m));
    }
    const E unknown("ZZZ");
    std::printf(" | ");
    Text(ToName(unknown));
    std::printf("/%s/%s\n", IsKnown(unknown) ? "known" : "unknown", from("ZZZ").has_value() ? "found" : "none");
}

void OpenCodes() {
    std::printf("Stat codes:");
    for (const h::Stat& m : h::kStatMembers) {
        const std::optional<uint16_t> code = ToCode(m);
        std::printf(" %d/%s", code ? static_cast<int>(*code) : -1, code ? Back(h::StatFromCode(*code), m) : "none");
    }
    const std::optional<h::Stat> alias = h::StatFromCode(9);
    std::printf(" | 9=");
    Text(alias ? ToName(*alias) : std::string_view("none"));
    std::printf(" 77=%s ZZZ=%s\n", h::StatFromCode(77).has_value() ? "found" : "none",
                ToCode(h::Stat("ZZZ")).has_value() ? "found" : "none");
}

// Retired prints IsRetired of each member (1 retired) and of a value that is no member or an unknown wire,
// then the live members' count and the first live one's name (DECISIONS 395).
template <typename E, std::size_t N, typename Live>
void Retired(const char* label, const std::array<E, N>& members, const Live& live, const E& other) {
    std::printf("%s retired:", label);
    for (const E& m : members) std::printf(" %d", IsRetired(m) ? 1 : 0);
    std::printf(" | other=%d live=%zu", IsRetired(other) ? 1 : 0, live.size());
    if (!live.empty()) {
        std::printf(" first=");
        Text(ToName(live[0]));
    }
    std::printf("\n");
}

}  // namespace

int main() {
    Closed("Tone", h::kToneMembers, &h::ToneFromWire);
    Closed("Element", h::kElementMembers, &h::ElementFromWire);
    Closed("Delta", h::kDeltaMembers, &h::DeltaFromWire);
    Closed("Extreme", h::kExtremeMembers, &h::ExtremeFromWire);
    Closed("Dup", h::kDupMembers, &h::DupFromWire);
    Closed("Nothing", h::kNothingMembers, &h::NothingFromWire);
    Closed("NoCode", h::kNoCodeMembers, &h::NoCodeFromWire);
    Closed("ShapeKind", h::kShapeKindMembers, &h::ShapeKindFromWire);
    Coded("Element", h::kElementMembers, &h::ElementFromCode, [](h::Element e) { return h::detail::ElementOrdinal(e); }, 2);
    Coded("Delta", h::kDeltaMembers, &h::DeltaFromCode, [](h::Delta e) { return h::detail::DeltaOrdinal(e); }, -1);
    Coded("Extreme", h::kExtremeMembers, &h::ExtremeFromCode, [](h::Extreme e) { return h::detail::ExtremeOrdinal(e); }, 5);
    Coded("NoCode", h::kNoCodeMembers, &h::NoCodeFromCode, [](h::NoCode e) { return h::detail::NoCodeOrdinal(e); }, 0);
    Open("Stat", h::kStatMembers, &h::StatFromWire);
    Open("Blank", h::kBlankMembers, &h::BlankFromWire);
    OpenCodes();
    Retired("Tone", h::kToneMembers, h::kToneLiveMembers, static_cast<h::Tone>(255));
    Retired("Element", h::kElementMembers, h::kElementLiveMembers, static_cast<h::Element>(77));
    Retired("Nothing", h::kNothingMembers, h::kNothingLiveMembers, static_cast<h::Nothing>(255));
    Retired("ShapeKind", h::kShapeKindMembers, h::kShapeKindLiveMembers, static_cast<h::ShapeKind>(255));
    Retired("Stat", h::kStatMembers, h::kStatLiveMembers, h::Stat("ZZZ"));
    Retired("Blank", h::kBlankMembers, h::kBlankLiveMembers, h::Blank("ZZZ"));
    Retired("OpenDup", h::kOpenDupMembers, h::kOpenDupLiveMembers, h::OpenDup("x"));
    std::printf("OpenDup live:");
    for (const h::OpenDup& m : h::kOpenDupLiveMembers) {
        std::printf(" ");
        Text(ToWire(m));
    }
    std::printf("\n");
    return 0;
}
