// Drives the generated pipeline code (CODEGEN.md §5.9, §5.11; CONFORMANCE.md §7, CPP-06):
// argv[1] holds good/, stale/ and an empty missing/. Prints one line per check that
// fails and returns their number.
#include "pipeline.gen.h"

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

}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    const std::string root = argv[1];
    using namespace sov::gen;
    Check(conformance::RunPipelineConformance() == 0, "RunPipelineConformance() == 0");
    Check(PipelineStore::Current() == nullptr, "no snapshot before the first Reload");

    std::string error;
    Check(PipelineStore::Reload(root + "/good", error), "Reload(good): " + error);
    std::shared_ptr<const PipelineSnapshot> snap = PipelineStore::Current();
    if (snap == nullptr) return failures + 1;
    const Potions& potions = snap->GetPotions();
    Check(potions.Len() == 2, "two potions");
    Check(potions.At(0).GetId() == "II_POT_HEAL_L" && potions.At(1).GetId() == "II_POT_HEAL_S", "source order");
    const Potion* large = potions.Find("II_POT_HEAL_L");
    Check(large != nullptr, "Find(II_POT_HEAL_L)");
    if (large == nullptr) return failures;
    Check(large->GetName() == "IDS_PROPITEM_TXT_POT_L", "name");
    Check(large->GetHeal() == 2500 && large->GetStack() == 20, "heal and stack");
    Check(large->GetCooldown().count() == 8000, "cooldown in ms");
    Check(large->IsStrong() && !potions.At(1).IsStrong(), "isStrong from $isStrong");
    Check(large->HealFor(200) == 200 && large->HealFor(9000) == 2500 && large->HealFor(-5) == 0, "HealFor");
    Check(potions.Find("II_POT_NONE") == nullptr, "an absent key is nullptr");
    Check(potions.All().size() == 2, "All()");

    // M2 acceptance 5: another schema is refused, naming both fingerprints; the old snapshot stays.
    error.clear();
    Check(!PipelineStore::Reload(root + "/stale", error), "Reload(stale) fails");
    std::printf("stale: %s\n", error.c_str());
    Check(error.find("pipeline.Potion@00000000") != std::string::npos, "the error names the file's schema");
    Check(error.find("pipeline.Potion@f750790e") != std::string::npos, "the error names the binary's schema");
    Check(PipelineStore::Current() == snap, "the old snapshot stays after a failed Reload");

    error.clear();
    Check(!PipelineStore::Reload(root + "/missing", error), "Reload(missing) fails");
    std::printf("missing: %s\n", error.c_str());
    Check(PipelineStore::Current() == snap, "the old snapshot stays after a missing file");

    std::string direct;
    Check(PipelineSnapshot::Load(root + "/good", direct) != nullptr, "PipelineSnapshot::Load");
    std::printf("failures: %d\n", failures);
    return failures;
}
