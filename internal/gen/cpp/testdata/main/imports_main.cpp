// Drives two packages, demo.app holding the types of demo.base (CODEGEN.md §2.8): argv[1]
// holds orders.json. Prints one line per failing check.
#include "demo/app/out/app.gen.h"

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
    using demo::base::Color;
    Check(demo::app::conformance::RunAppConformance() == 0, "RunAppConformance() == 0");
    Check(demo::app::Warm(Color::red) && !demo::app::Warm(Color::green), "a foreign enum parameter");
    std::string error;
    std::shared_ptr<const demo::app::Orders> orders = demo::app::Orders::Load(std::string(argv[1]) + "/orders.json", error);
    Check(orders != nullptr, "Orders::Load: " + error);
    if (orders == nullptr) return failures;
    const demo::app::Order* o1 = orders->Find("o1");
    const demo::app::Order* o2 = orders->Find("o2");
    Check(o1 != nullptr && o2 != nullptr, "Find");
    if (o1 == nullptr || o2 == nullptr) return failures;
    Check(o1->GetColor() == Color::blue && o1->GetAt().GetX() == 3 && o1->GetTrail().size() == 2, "foreign enum and record");
    Check(o1->GetPaint().AsSolid() != nullptr && o1->GetPaint().AsSolid()->GetTint() == Color::green, "foreign inline variant");
    Check(o2->GetPaint().GetKind() == demo::base::PaintKind::clear && o1->GetAlt() == nullptr && o2->GetAlt()->GetX() == 9,
          "foreign fieldless case and optional record");
    Check(o1->Hue(Color::green) == 2 && o2->Hue(Color::blue) == 6, "a lookup over a foreign enum");
    // A foreign dependent type, decoded by its own package's Decode<Alias> (CODEGEN.md §2.8, §5.6).
    Check(o1->GetShade() == nullptr, "a Never branch is none");
    Check(o2->GetShade() != nullptr && o2->GetShade()->GetBranch() == demo::base::ShadeBranch::red &&
              *o2->GetShade()->AsRed() == "warm" && !o2->GetShade()->AsGreen(),
          "a foreign dependent type");
    std::printf("failures: %d\n", failures);
    return failures;
}
