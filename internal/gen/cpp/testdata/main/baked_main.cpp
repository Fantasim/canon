// The baked fixture: constexpr lookups in constant expressions (DECISIONS 293), the baked data,
// its resolved refs and id enums at run time (CODEGEN.md §5.3, §5.8, §5.9, §5.10).
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <string>
#include <string_view>

#include "board.gen.h"

namespace b = demo::board;

template <int64_t N>
struct Version {
    static constexpr int64_t value = N;
};

template <b::ColumnId C>
struct Shown {
    static constexpr b::ColumnId value = C;
};

template <b::Tone T>
struct Toned {
    static constexpr b::Tone value = T;
};

template <bool B>
struct Flag {
    static constexpr bool value = B;
};

static_assert(b::VersionOf(b::Element::FIRE) == 1, "a @codes enum's ordinal is its position");
static_assert(b::VersionOf(b::Element::EARTH) == 2, "a retired member keeps its cell");
static_assert(b::LabelOf(b::Tone::loud) == std::string_view("Loud"), "a String result is a std::string_view");
static_assert(b::IsOpen(b::StatusId::open) && !b::IsOpen(b::StatusId::taken), "a table id parameter");
static_assert(b::ToneOf(b::Element::FIRE) == b::Tone::loud, "an enum result");
static_assert(b::ColumnOf(b::StatusId::closed) == b::ColumnId::done, "a table id result");
static_assert(b::DelayOf(true) == std::chrono::milliseconds(2000), "a Duration result");
static_assert(b::Greeting() == std::string_view("hi \"you\""), "a precomputed String");
static_assert(Version<b::VersionOf(b::Element::WATER)>::value == 2, "an Int as a template argument");
static_assert(Shown<b::ColumnOf(b::StatusId::open)>::value == b::ColumnId::todo, "a table id as a template argument");
static_assert(Toned<b::ToneOf(b::Element::WATER)>::value == b::Tone::quiet, "an enum as a template argument");
static_assert(Flag<b::IsOpen(b::StatusId::closed)>::value == false, "a Bool as a template argument");
static_assert(b::MoodOf(b::Tone::loud) == std::string_view("angry"), "a string literal union result is a std::string_view");

int main() {
    const b::Statuses& statuses = b::GetStatuses();
    for (const b::Status& s : statuses.All()) {
        std::string next;
        for (const b::Status* n : s.GetNext()) next += std::string(b::ToWire(n->GetId())) + ",";
        const b::Point* at = s.GetAt();
        std::printf("%s %s col=%s next=%s retired=%d final=%d rank=%lld at=%lld reward=%d for=%s\n",
                    std::string(b::ToWire(s.GetId())).c_str(), s.GetLabel().c_str(), s.GetColumn().GetTitle().c_str(),
                    next.c_str(), s.GetRetired() ? 1 : 0, s.Final() ? 1 : 0, static_cast<long long>(s.Rank(b::Tone::loud)),
                    static_cast<long long>(at ? at->GetX() : -1), static_cast<int>(s.GetReward().GetKind()),
                    s.ColumnFor(b::StatusId::taken).GetTitle().c_str());
    }
    const auto id = b::StatusIdFromWire("closed");
    std::printf("find=%s get=%s id=%d none=%d col=%s\n", statuses.Find("taken")->GetLabel().c_str(),
                statuses.Get(b::StatusId::closed).GetLabel().c_str(), id ? static_cast<int>(*id) : -1,
                b::ColumnIdFromWire("nope") ? 1 : 0, b::GetColumns().Get(b::ColumnId::done).GetTitle().c_str());
    std::printf("initial=%s key=%d limit=%lld tags=%zu home=%lld prize=%lld\n", b::GetInitialStatus().GetLabel().c_str(),
                static_cast<int>(b::GetInitialStatusKey()), static_cast<long long>(b::GetLimit()), b::GetTags().size(),
                static_cast<long long>(b::GetHome().GetY()), static_cast<long long>(b::GetPrize().AsCoins()->GetAmount()));
    const b::Column* maybe = b::MaybeColumn(b::Tone::loud);
    std::printf("nextOf=%zu maybe=%s none=%d tags=%zu primes=%zu version=%lld doubled=%lld\n",
                b::NextOf(b::StatusId::taken).size(), maybe ? maybe->GetTitle().c_str() : "-", b::MaybeColumn(b::Tone::quiet) ? 1 : 0,
                b::TagsOf(b::Tone::loud).size(), b::Primes().size(),
                static_cast<long long>(b::VersionOf(static_cast<b::Element>(2))), static_cast<long long>(b::Doubled(b::Element::WATER, 4)));
    const b::Shelf& shelf = b::GetShelf();
    std::printf("points=%zu far=%lld shelf=%zu b=%d crew=%lld name=%s pick=%s\n", b::GetPoints().Len(),
                static_cast<long long>(b::GetPoints().Find("far")->GetX()), shelf.GetSlots().Len(),
                shelf.GetSlots().Find("b")->GetRetired() ? 1 : 0, static_cast<long long>(b::CrewOf(b::Tone::loud).GetSize()),
                b::CrewName(b::Tone::quiet).c_str(), std::string(b::ToWire(b::PickColumn(0))).c_str());
    std::printf("failures=%d\n", b::conformance::RunBoardConformance());
    return 0;
}
