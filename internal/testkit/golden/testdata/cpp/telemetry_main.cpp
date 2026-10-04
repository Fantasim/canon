// Driver for TestTelemetryCppCompiles: uses examples/telemetry's per-event metadata at
// compile time (DECISIONS 293, CODEGEN.md §5.10: header-inline constexpr lookups over an
// enum), as Sovereign's packer and ring set do, then prints a few values at run time.
#include <array>
#include <cstdint>
#include <cstdio>
#include <string>

#include "telemetry.gen.h"

using Telemetry::EventType;
using Telemetry::Producer;
using Telemetry::Tier;

// The wire byte is the code (CODEGEN.md §5.2): the dispatch table indexes by it.
static_assert(static_cast<std::uint8_t>(EventType::farm_events) == 151, "farm_events is 151");
static_assert(static_cast<std::uint8_t>(EventType::world_boss_spawns) == 156, "a tombstone keeps its id");

// Metadata in static_assert (DECISIONS 293).
static_assert(Telemetry::VersionOf(EventType::farm_events) == 2, "farm_events v2");
static_assert(Telemetry::VersionOf(EventType::item_created) == 5, "item_created v5");
static_assert(Telemetry::TierOf(EventType::penya_drops) == Tier::HOT, "penya_drops is HOT");
static_assert(Telemetry::HasInstanceId(EventType::item_created), "item_created has item_instance_id");
static_assert(!Telemetry::HasInstanceId(EventType::farm_events), "farm_events has none");
static_assert(Telemetry::Emits(EventType::server_lifecycle, Producer::LoginServer), "the spine is everywhere");
static_assert(!Telemetry::Emits(EventType::sovereign_events, Producer::WorldServer), "CoreServer only");
static_assert(Telemetry::IsRetired(EventType::mail_item_received), "20 is a tombstone");
static_assert(Telemetry::TableOf(EventType::trade_penya) == "trade_penya", "the member names the table");
static_assert(Telemetry::CreateTableSql(EventType::farm_events).substr(0, 39) ==
                  "CREATE TABLE IF NOT EXISTS farm_events ",
              "the DDL is a constant");

// The packed struct the producer serializes; its size is pinned by the catalogue.
#pragma pack(push, 1)
struct FarmEvent {
    static constexpr EventType TYPE = EventType::farm_events;
    std::uint64_t timestamp_ns;
    std::uint32_t player_id;
    std::uint8_t kind;
    std::uint32_t slot_index;
    std::uint32_t type_id;
    std::uint32_t item_count;
    std::uint32_t target_player_id;
    std::uint64_t fact_id;
};
struct PenyaDropEvent {
    static constexpr EventType TYPE = EventType::penya_drops;
    std::uint64_t timestamp_ns;
    std::uint32_t player_id;
    std::int64_t amount;
    std::uint32_t mob_index;
};
#pragma pack(pop)
static_assert(sizeof(FarmEvent) == Telemetry::WireSize(FarmEvent::TYPE), "FarmEvent drifted from the catalogue");
static_assert(sizeof(PenyaDropEvent) == Telemetry::WireSize(PenyaDropEvent::TYPE), "PenyaDropEvent drifted");

// Metadata as template arguments: the ring's drop policy parameterised by tier (a tier is a
// drop and WARN policy, never a ring size).
template <Tier T>
struct DropPolicy {
    static constexpr bool kWarnOnce = T == Tier::HOT;
    static constexpr bool kHasRing = T != Tier::READONLY;
};
template <class E>
using PolicyOf = DropPolicy<Telemetry::TierOf(E::TYPE)>;
static_assert(!PolicyOf<FarmEvent>::kWarnOnce && PolicyOf<FarmEvent>::kHasRing, "COOL has a ring and is not HOT");
static_assert(PolicyOf<PenyaDropEvent>::kWarnOnce, "HOT: drops expected, warned once");

// And as an array bound and an if constexpr condition.
static std::array<int, Telemetry::VersionOf(EventType::item_created)> perVersion{};

template <class E>
constexpr bool IsHot() {
    if constexpr (Telemetry::TierOf(E::TYPE) == Tier::HOT) {
        return true;
    } else {
        return false;
    }
}
static_assert(IsHot<PenyaDropEvent>() && !IsHot<FarmEvent>(), "if constexpr on the tier");

int main() {
    for (EventType e : Telemetry::kEventTypeMembers) {
        std::printf("%u %s v%u %s size=%lld retired=%d\n", static_cast<unsigned>(e),
                    std::string(Telemetry::TableOf(e)).c_str(), static_cast<unsigned>(Telemetry::VersionOf(e)),
                    std::string(Telemetry::ToName(Telemetry::TierOf(e))).c_str(),
                    static_cast<long long>(Telemetry::WireSize(e)), Telemetry::IsRetired(e) ? 1 : 0);
    }
    const Telemetry::Event* farm = Telemetry::GetEvents().Find(EventType::farm_events);
    if (farm == nullptr) {
        return 1;
    }
    const Telemetry::Column* cost = farm->GetColumns().Find("cost");
    std::printf("farm_events columns=%zu cost.retiredIn=%lld ledger=%s until=%lld slots=%zu\n",
                farm->GetColumns().Len(), static_cast<long long>(cost->GetRetiredIn().value_or(0)),
                farm->GetLedger().at(0).GetToken().c_str(),
                static_cast<long long>(farm->GetLedger().at(0).GetUntilVersion().value_or(0)), perVersion.size());
    std::printf("%s\n", std::string(Telemetry::CreateTableSql(EventType::farm_events)).c_str());
    return 0;
}
