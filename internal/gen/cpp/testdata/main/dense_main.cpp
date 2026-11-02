// Drives demo.dense and demo.denseuse (DECISIONS 430, CODEGEN.md §5.2): the ordinal of an @codes
// enum whose codes are dense in declaration order is a subtraction, whatever the type's range,
// and every helper answers as the search did: a position for a member, the member count for a
// value that is no member, below the first code, past the last, at the type's minimum and maximum.
#include <array>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <initializer_list>
#include <limits>
#include <optional>
#include <string>
#include <string_view>
#include <type_traits>

#include "demo/dense/out/dense.gen.h"
#include "demo/denseuse/out/denseuse.gen.h"

namespace d = demo::dense;
namespace u = demo::denseuse;

constexpr long long kMin = std::numeric_limits<long long>::min();
constexpr long long kMax = std::numeric_limits<long long>::max();

// The ordinal is constexpr: the declaration position of a member, the member count for any other value.
static_assert(d::detail::LevelOrdinal(d::Level::m0) == 0, "-128 is the first Level");
static_assert(d::detail::LevelOrdinal(d::Level::m2) == 2, "-126 is the third Level");
static_assert(d::detail::LevelOrdinal(static_cast<d::Level>(-125)) == 3, "-125 is past the last Level");
static_assert(d::detail::LevelOrdinal(static_cast<d::Level>(127)) == 3, "127 is past the last Level");
static_assert(d::detail::EraOrdinal(static_cast<d::Era>(5999)) == 3, "5999 is below the first Era");
static_assert(d::detail::EraOrdinal(d::Era::m1) == 1, "6001 is the second Era");
static_assert(d::detail::EraOrdinal(static_cast<d::Era>(6003)) == 3, "6003 is past the last Era");
static_assert(d::detail::EraOrdinal(static_cast<d::Era>(0)) == 3, "0 is below the first Era");
static_assert(d::detail::WideOrdinal(d::Wide::m1) == 1, "INT64_MAX is the second Wide");
static_assert(d::detail::WideOrdinal(static_cast<d::Wide>(kMin)) == 2, "INT64_MIN is no Wide");
static_assert(d::detail::WideOrdinal(static_cast<d::Wide>(kMax - 2)) == 2, "INT64_MAX - 2 is no Wide");
static_assert(d::detail::FloorOrdinal(d::Floor::m0) == 0, "INT64_MIN is the first Floor");
static_assert(d::detail::FloorOrdinal(static_cast<d::Floor>(kMax)) == 2, "INT64_MAX is no Floor");
static_assert(d::detail::StartOrdinal(d::Start::m3) == 3, "3 is the fourth Start");
static_assert(d::detail::StartOrdinal(static_cast<d::Start>(4)) == 4, "4 is past the last Start");
static_assert(d::detail::SingleOrdinal(static_cast<d::Single>(0)) == 1, "0 is below the only Single");
static_assert(d::detail::SingleOrdinal(static_cast<d::Single>(2)) == 1, "2 is past the only Single");
static_assert(d::detail::GapOrdinal(d::Gap::m1) == 1, "INT64_MIN is the second Gap");
static_assert(d::detail::GapOrdinal(static_cast<d::Gap>(0)) == 2, "0 is no Gap");
static_assert(d::detail::VoidOrdinal(static_cast<d::Void>(0)) == 0, "Void has no member");

// A lookup of another package over the dense enums is constexpr too.
static_assert(u::WeightOf(d::Era::m0) == 4, "the first Era");
static_assert(u::WeightOf(d::Era::m2) == 9, "the third Era");
static_assert(u::FloorOf(d::Floor::m0) == -7, "the Floor at INT64_MIN");
static_assert(u::FloorOf(d::Floor::m1) == 8, "the Floor above it");

namespace {

void Text(std::string_view s) { std::printf("%.*s", static_cast<int>(s.size()), s.data()); }

// Dump prints each member's name, wire, ordinal and whether FromCode and FromWire give it back, then
// what a value that is no member answers: its ordinal, its name and wire, FromCode.
template <typename E, std::size_t N, typename FromCode, typename FromWire, typename Ordinal>
void Dump(const char* label, const std::array<E, N>& members, FromCode fromCode, FromWire fromWire, Ordinal ordinal,
          std::initializer_list<long long> others) {
    using Code = std::underlying_type_t<E>;
    std::printf("%s:", label);
    for (const E m : members) {
        const auto byCode = fromCode(static_cast<Code>(m));
        const auto byWire = fromWire(ToWire(m));
        std::printf(" ");
        Text(ToName(m));
        std::printf("/");
        Text(ToWire(m));
        std::printf("/%zu/%s/%s/%d", ordinal(m), byCode && *byCode == m ? "code" : "bad", byWire && *byWire == m ? "wire" : "bad",
                    IsRetired(m) ? 1 : 0);
    }
    std::printf(" |");
    for (const long long other : others) {
        const Code code = static_cast<Code>(other);
        const E e = static_cast<E>(code);
        std::printf(" %lld=%zu/[", other, ordinal(e));
        Text(ToName(e));
        std::printf("|");
        Text(ToWire(e));
        std::printf("]/%s/%d", fromCode(code) ? "found" : "none", IsRetired(e) ? 1 : 0);
    }
    std::printf("\n");
}

}  // namespace

int main() {
    Dump("Level", d::kLevelMembers, d::LevelFromCode, d::LevelFromWire, d::detail::LevelOrdinal, {-125, 0, 127});
    Dump("Era", d::kEraMembers, d::EraFromCode, d::EraFromWire, d::detail::EraOrdinal, {5999, 6003, 0, 65535});
    Dump("Wide", d::kWideMembers, d::WideFromCode, d::WideFromWire, d::detail::WideOrdinal, {kMin, 0, kMax - 2});
    Dump("Floor", d::kFloorMembers, d::FloorFromCode, d::FloorFromWire, d::detail::FloorOrdinal, {kMin + 2, 0, kMax});
    Dump("Start", d::kStartMembers, d::StartFromCode, d::StartFromWire, d::detail::StartOrdinal, {4, 255});
    Dump("Single", d::kSingleMembers, d::SingleFromCode, d::SingleFromWire, d::detail::SingleOrdinal, {0, 2, 255});
    Dump("Spelled", d::kSpelledMembers, d::SpelledFromCode, d::SpelledFromWire, d::detail::SpelledOrdinal, {0, 3});
    Dump("Gap", d::kGapMembers, d::GapFromCode, d::GapFromWire, d::detail::GapOrdinal, {0, kMin + 1});
    Dump("Skip", d::kSkipMembers, d::SkipFromCode, d::SkipFromWire, d::detail::SkipOrdinal, {0, 2, 4});
    Dump("Void", d::kVoidMembers, d::VoidFromCode, d::VoidFromWire, d::detail::VoidOrdinal, {0, 255});
    std::printf("lookups: %lld %lld %lld %lld\n", static_cast<long long>(u::WeightOf(d::Era::m1)), static_cast<long long>(u::WeightOf(d::Era::m2)),
                static_cast<long long>(u::FloorOf(d::Floor::m0)), static_cast<long long>(u::FloorOf(d::Floor::m1)));
    return 0;
}
