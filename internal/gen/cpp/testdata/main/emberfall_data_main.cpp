// Drives game.world in data mode: its readers read game.core's classes and build them through
// game.core's make hooks (CODEGEN.md §2.8, §5.9, §5.14, §7.6; DECISIONS 323). argv[1] holds
// zones.json, bands.json and start.json; argv[2] a bands.json with a key naming no entry.
// Prints one line per failing check, then the bad file's error.
#include "game/camp/out/camp.gen.h"
#include "game/world/out/world.gen.h"

#include <cstdio>
#include <memory>
#include <string>

namespace {

int failures = 0;

void Check(bool ok, const std::string& what) {
    if (!ok) {
        std::printf("FAIL %s\n", what.c_str());
        ++failures;
    }
}

void CheckZones(const game::world::Zones& zones) {
    using game::core::StatusId;
    const game::world::Zone* z = zones.Find("forest");
    Check(z != nullptr, "Find");
    if (z == nullptr) return;
    const game::core::LevelRange& levels = z->GetLevels();
    Check(levels.GetMin() == 1 && levels.GetMax() == 5 && levels.GetLabels().size() == 2, "a field");
    Check(levels.GetStatusKey() == StatusId::open && levels.GetStatus().GetLabel() == "Open", "a getter the owner's hook resolved");
    Check(levels.Width() == 4, "the owner's stored result");
    Check(z->GetTrail().size() == 1 && z->GetTrail()[0].GetStatus().GetLabel() == "Closed", "a list element");
    const game::core::LevelRange* east = z->GetByName().Find("east");
    Check(east != nullptr && east->GetMax() == 9, "a map value");
    Check(z->GetReward().AsCoins() != nullptr && z->GetReward().AsCoins()->GetAmount() == 3, "a variant");
    Check(z->GetParam().AsLoud() == 7 && !z->GetParam().AsSoft(), "a dependent type");
    const game::world::MarkerRow* well = z->GetMarkers().Find("well");
    Check(well != nullptr && well->GetCode() == "w" && well->GetId() == "well" && well->GetRetired(), "a table field's row");
    const game::world::MarkerRow* gate = z->GetMarkers().Find("gate");
    Check(gate != nullptr && gate->GetNext() != nullptr && gate->GetNext()->GetCode() == "n", "a boxed member of a row's base");
    Check(z->GetBonuses().size() == 2 && z->GetBonuses()[1].GetStatKey() == "b" && z->GetBonuses()[1].GetStat().GetLabel() == "Bow", "a pairs field");
    const game::core::Item* sword = z->GetKit().GetItems().Find("sword");
    Check(sword != nullptr && sword->GetName() == "Sword" && sword->GetId() == "sword" && !sword->GetRetired(), "the owner's table, through its entry hook");
    const game::core::ChimeRow* c1 = z->GetBell().GetChimes().Find("c1");
    Check(c1 != nullptr && c1->GetPitch() == 440 && c1->GetId() == "c1" && c1->GetRetired(), "the owner's row class, through its row hook");
    const game::core::Gear& gear = z->GetGear();
    Check(gear.GetKindKey() == "a" && gear.GetKind().GetLabel() == "Axe", "a field resolved into the owner's keyed list");
    Check(gear.Best().GetLabel() == "Bow" && gear.KindFor(game::tone::Tone::soft) != nullptr &&
              gear.KindFor(game::tone::Tone::soft)->GetLabel() == "Bow" && gear.KindFor(game::tone::Tone::loud) == nullptr,
          "a stored result and lookup cells resolved into the owner's keyed list");
    Check(z->Band().GetMax() == 20 && z->Band().GetStatus().GetLabel() == "Closed", "a stored result");
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 3) return 100;
    const std::string dir(argv[1]);
    std::string error;
    auto zones = game::world::Zones::Load(dir + "/zones.json", error);
    Check(zones != nullptr, "Zones::Load: " + error);
    if (zones != nullptr) CheckZones(*zones);
    // game.camp, emitted into game.world's namespace, reads game.core's LevelRange with its own reader.
    std::shared_ptr<const game::world::Camp> camp = game::world::Camp::Load(dir + "/camp.json", error);
    Check(camp != nullptr && camp->GetLevels().GetMax() == 9 && camp->GetLevels().GetStatus().GetLabel() == "Closed",
          "two packages of one namespace read one foreign class: " + error);
    auto bands = game::world::Bands::Load(dir + "/bands.json", error);
    Check(bands != nullptr, "Bands::Load: " + error);
    if (bands != nullptr) {
        const game::world::LevelRangeRow* high = bands->Find("high");
        Check(high != nullptr && high->GetId() == "high" && high->GetRetired() && high->Width() == 4, "a table value's row");
    }
    std::shared_ptr<const game::core::LevelRange> start = game::world::LoadStart(dir + "/start.json", error);
    Check(start != nullptr && start->GetMax() == 2 && start->GetTone() == game::tone::Tone::loud, "the free Load<V>: " + error);
    error.clear();
    Check(game::world::Bands::Load(std::string(argv[2]) + "/bands.json", error) == nullptr, "a key naming no entry is refused");
    std::printf("bad: %s\n", error.c_str());
    for (const char* file : {"/zones.json", "/cells.json", "/pairs.json"}) {
        error.clear();
        Check(game::world::Zones::Load(std::string(argv[2]) + file, error) == nullptr, std::string("a key naming no entry is refused: ") + file);
        std::printf("bad: %s\n", error.c_str());
    }
    std::printf("failures: %d\n", failures);
    return failures;
}
