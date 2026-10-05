// Decodes a game.world Zone through its types-mode public decoder, whose readers build game.core's
// classes through game.core's make hooks (CODEGEN.md §2.8, §5.13, §5.14). Prints one line per
// failing check, then the error of a key naming no entry.
#include "game/world/out/world.gen.h"

#include <nlohmann/json.hpp>

#include <cstdio>
#include <optional>
#include <string>

namespace {

int failures = 0;

void Check(bool ok, const std::string& what) {
    if (!ok) {
        std::printf("FAIL %s\n", what.c_str());
        ++failures;
    }
}

const char* const kZone = R"({"levels": {"min": 1, "max": 5, "labels": ["a"], "tone": "soft", "status": "open"},
  "trail": [], "reward": {"kind": "nothing"}, "tone": "soft", "param": "hi",
  "markers": {"gate": {"code": "g"}}, "stat0": "a", "value0": 2,
  "kit": {"items": {"sword": {"name": "Sword"}}}, "bell": {"chimes": {"c1": {"pitch": 440}}}})";

}  // namespace

int main() {
    std::string error;
    std::optional<game::world::Zone> z = game::world::Zone::Decode(nlohmann::json::parse(kZone, nullptr, false), error);
    Check(z.has_value(), "Zone::Decode: " + error);
    if (z) {
        Check(z->GetLevels().GetMax() == 5 && z->GetLevels().GetStatus().GetLabel() == "Open", "a field, resolved by the owner's hook");
        Check(z->GetReward().GetKind() == game::core::RewardKind::nothing, "a fieldless case");
        Check(z->GetParam().AsSoft() != nullptr && *z->GetParam().AsSoft() == "hi", "a dependent type");
        Check(z->GetMarkers().Find("gate") != nullptr && z->GetMarkers().Find("gate")->GetId() == "gate", "a table field's row");
        Check(z->GetBonuses().size() == 1 && z->GetBonuses()[0].GetValue() == 2 && z->GetBonuses()[0].GetStat().GetLabel() == "Axe", "a pairs field");
        Check(z->GetKit().GetItems().Find("sword")->GetId() == "sword", "the owner's table, through its entry hook");
        Check(z->GetBell().GetChimes().Find("c1")->GetPitch() == 440, "the owner's row class, through its row hook");
    }
    std::string bad(kZone);
    bad.replace(bad.find("\"open\""), 6, "\"gone\"");
    Check(!game::world::Zone::Decode(nlohmann::json::parse(bad, nullptr, false), error), "a key naming no entry is refused");
    std::printf("bad: %s\n", error.c_str());
    std::printf("failures: %d\n", failures);
    return failures;
}
