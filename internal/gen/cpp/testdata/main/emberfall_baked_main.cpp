// Drives game.world baked: every value of game.core it holds was written through game.core's
// make hooks (CODEGEN.md §2.8, §5.9, §5.14; DECISIONS 323). Prints one line per failing check.
#include "game/world/out/world.gen.h"

#include <cstdio>
#include <string>

namespace {

int failures = 0;

void Check(bool ok, const std::string& what) {
    if (!ok) {
        std::printf("FAIL %s\n", what.c_str());
        ++failures;
    }
}

}  // namespace

int main() {
    using game::core::StatusId;
    using game::tone::Tone;
    namespace world = game::world;
    const world::Zone& z = world::GetZones().Get(world::ZoneId::forest);
    const game::core::LevelRange& levels = z.GetLevels();
    Check(levels.GetMin() == 1 && levels.GetMax() == 5 && levels.GetLabels().size() == 2 && levels.GetLabels()[1] == "b", "a field");
    Check(levels.GetTone() == Tone::soft && levels.GetStatusKey() == StatusId::open, "a key into the owner's table");
    Check(levels.GetStatus().GetLabel() == "Open", "a getter the owner's hook resolved");
    Check(levels.Width() == 4, "the owner's stored result");
    Check(z.GetTrail().size() == 1 && z.GetTrail()[0].GetStatus().GetLabel() == "Closed", "a list element");
    const game::core::LevelRange* east = z.GetByName().Find("east");
    Check(east != nullptr && east->GetMax() == 9 && east->Width() == 6, "a map value");
    Check(z.GetReward().AsCoins() != nullptr && z.GetReward().AsCoins()->GetAmount() == 3, "a variant");
    Check(z.GetParam().GetBranch() == game::core::ParamBranch::loud && z.GetParam().AsLoud() == 7, "a dependent type");
    const world::MarkerRow* gate = z.GetMarkers().Find("gate");
    const world::MarkerRow* well = z.GetMarkers().Find("well");
    Check(gate != nullptr && gate->GetCode() == "g" && gate->GetId() == "gate" && !gate->GetRetired(), "a table field's row");
    Check(well != nullptr && well->GetRetired() && well->GetNext() == nullptr, "a retired row of a table field");
    Check(gate != nullptr && gate->GetNext() != nullptr && gate->GetNext()->GetCode() == "n", "a boxed member of a row's base");
    Check(z.GetBonuses().size() == 2 && z.GetBonuses()[0].GetStat().GetLabel() == "Axe" && z.GetBonuses()[1].GetValue() == 1, "a pairs field");
    const game::core::Item* sword = z.GetKit().GetItems().Find("sword");
    Check(sword != nullptr && sword->GetName() == "Sword" && sword->GetId() == "sword" && !sword->GetRetired(), "the owner's table, through its entry hook");
    const game::core::ChimeRow* c1 = z.GetBell().GetChimes().Find("c1");
    Check(c1 != nullptr && c1->GetPitch() == 440 && c1->GetId() == "c1" && c1->GetRetired(), "the owner's row class, through its row hook");
    const game::core::Gear& gear = z.GetGear();
    Check(gear.GetKind().GetLabel() == "Axe" && gear.Best().GetLabel() == "Bow", "a field and a stored result the owner's hook resolved");
    Check(gear.KindFor(Tone::soft) != nullptr && gear.KindFor(Tone::soft)->GetCode() == "b" && gear.KindFor(Tone::loud) == nullptr,
          "lookup cells the owner's hook resolved");
    const world::Patrol patrol;
    Check(patrol.GetN() == 0, "a lookup over the owner's ids with no receiver: an empty domain");
    Check(z.Band().GetMin() == 10 && z.Band().GetStatus().GetLabel() == "Closed" && z.Band().Width() == 10, "a stored result");
    const world::LevelRangeRow& low = world::GetBands().Get(world::LevelRangeId::low);
    const world::LevelRangeRow& high = world::GetBands().At(1);
    Check(low.GetId() == world::LevelRangeId::low && !low.GetRetired() && low.GetMax() == 3, "a table value's row");
    Check(high.GetId() == world::LevelRangeId::high && high.GetRetired() && high.GetStatus().GetLabel() == "Closed", "a retired row");
    const game::core::LevelRange& asRecord = low;
    Check(asRecord.Width() == 2, "a row is its record");
    Check(world::GetStart().GetMax() == 2 && world::GetStart().GetTone() == Tone::loud, "a root value");
    Check(world::START_TONE == Tone::loud, "a constant of a reached package's enum");
    static_assert(world::BandWidth(StatusId::closed) == 2, "a constexpr lookup over the owner's id enum");
    Check(world::BandWidth(StatusId::open) == 1, "a lookup parameter");
    std::printf("failures: %d\n", failures);
    return failures;
}
