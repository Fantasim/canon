// Drives the generated demo.shop code: every construct of the constructs fixture, read back
// from argv[1]/good (CODEGEN.md §5, §7.6). Prints one line per failing check.
#include "shop.defines.gen.h"
#include "shop.gen.h"

#include <cstdio>
#include <memory>
#include <string>
#include <type_traits>

namespace {

int failures = 0;

void Check(bool ok, const std::string& what) {
    if (!ok) {
        std::printf("FAIL %s\n", what.c_str());
        ++failures;
    }
}

using namespace demo::shop;

// Containers and the snapshot move but never copy; records copy (log-2026-09-24, round 3).
static_assert(!std::is_copy_constructible_v<Items> && !std::is_copy_assignable_v<ShopSnapshot>);
static_assert(std::is_move_constructible_v<Items> && std::is_move_assignable_v<ShopSnapshot>);
static_assert(std::is_copy_constructible_v<Item> && std::is_copy_assignable_v<Shelf>);

void CheckStatics() {
    Check(conformance::RunShopConformance() == 0, "RunShopConformance() == 0");
    Check(MAX_ITEMS == 100 && RATE == 0.5 && GREETING == "hi \"there\"\n", "scalar constants");
    Check(COOLDOWN.count() == 90000 && WAITS.size() == 2 && WAITS[1].count() == 2000, "duration constants");
    Check(PRIMES.size() == 3 && PRIMES[2] == 5, "list constant");
    Check(DEFAULT_TONE == Tone::info && TONES.size() == 2 && TONES[1] == Tone::series_1, "enum constants");
    Check(kToneMembers.size() == 4 && kToneMembers[3] == Tone::legacy, "kToneMembers");
    Check(ToWire(Tone::series_1) == "series-1" && ToName(Tone::series_1) == "series_1", "ToWire and ToName");
    Check(ToneFromWire("series-1") == Tone::series_1 && !ToneFromWire("series_1").has_value(), "ToneFromWire");
    Check(ElementFromCode(300) == Element::WATER && !ElementFromCode(2).has_value(), "ElementFromCode");
    Check(LIMITS.Len() == 2 && LIMITS.At(0).first == "b" && LIMITS.Find("a") != nullptr && *LIMITS.Find("a") == 1, "map constant");
    Check(delete_, "a reserved constant name gets a _ suffix");
    Check(IK1_WEAPON == 1 && IK1_GENERAL == 4 && static_cast<uint32_t>(Kind1::GENERAL) == 4 && kKind1Members.size() == 2,
          "@cpp(defines:) beside its enum");
    Check(Echo("a b") == "a b" && PickShelf(false) == "back" && Inc(41) == 42 && Unit(0.25) == 0.25, "more package fns");
    Check(static_cast<uint16_t>(Element::WATER) == 300, "a @codes enum's value is its code");
    Check(ToWire(RewardKind::nothing) == "none" && RewardKindFromWire("gold") == RewardKind::Coins && ToName(RewardKind::Coins) == "gold", "kind enum, a case override");
    Check(Mix(2, 3, 10, 2) == 11 && Label(Tone::info, 5) == "info:5" && IsLoud(Tone::warning), "package fns");
}

void CheckItems(const Items& items) {
    Check(items.Len() == 3, "three items");
    const Item* sword = items.Find("sword");
    const Item* anvil = items.FindByCode(7);
    Check(sword != nullptr && anvil != nullptr && items.FindByCode(12) == &items.At(2), "Find and FindByCode");
    Check(items.FindByCode(8) == nullptr && items.Find("gem") == nullptr, "absent keys");
    if (sword == nullptr || anvil == nullptr) return;
    Check(anvil->GetId() == "anvil" && anvil->GetRetired() && !sword->GetRetired(), "$id and $retired");
    Check(sword->GetCode() == 300 && sword->GetLabel() == "Sword" && sword->GetPrice() == 12.5, "scalars");
    Check(sword->GetWeight() == 3.25f && sword->GetTradable() && !anvil->GetTradable(), "Float32 and @json(int)");
    Check(sword->GetTone() == Tone::series_1 && anvil->GetTone() == Tone::legacy, "enum by wire");
    Check(sword->GetElement() == Element::WATER && anvil->GetElement() == Element::FIRE, "enum by code");
    Check(sword->GetDelay().count() == 2000 && items.At(2).GetDelay().count() == 60000, "duration in seconds");
    Check(sword->GetNote() == nullptr && anvil->GetNote() != nullptr && *anvil->GetNote() == "heavy", "none marker");
    Check(!sword->GetBonus().has_value() && anvil->GetBonus() == -2, "optional Int");
    Check(sword->GetOrigin() != nullptr && sword->GetOrigin()->GetX() == -3 && anvil->GetOrigin() == nullptr, "optional record");
    Check(sword->GetTags().size() == 2 && sword->GetTags()[1] == "metal" && anvil->GetTags().empty(), "list of strings");
    Check(sword->GetPath().size() == 2 && sword->GetPath()[1].GetY() == 4, "list of records");
    Check(sword->GetShelfKey() != nullptr && *sword->GetShelfKey() == "front" && anvil->GetShelfKey() == nullptr, "optional ref key");
    Check(sword->GetLegacyMax() == 9 && anvil->GetLegacyMax() == -1, "@json(path:)");
    Check(!sword->Heavy() && anvil->Heavy(), "precomputed $heavy");
    Check(sword->Nick() != nullptr && *sword->Nick() == "blade" && anvil->Nick() == nullptr, "optional precomputed $nick");
    Check(sword->Discounted(10) == 270 && anvil->Worth(7) == 7 && sword->Worth(7) == 0, "translated methods");
    Check(sword->Rank(Tone::series_1, true) == 21 && anvil->Rank(Tone::warning, false) == 100 &&
              items.At(2).Rank(Tone::legacy, true) == 231, "finite-parameter method");
    Check(sword->LabelIn(Element::FIRE) != nullptr && *sword->LabelIn(Element::FIRE) == "fire" &&
              sword->LabelIn(Element::WATER) == nullptr && *anvil->LabelIn(Element::WATER) == "wet", "optional table cells");
    Check(sword->Related() != nullptr && sword->Related()->size() == 2 && (*sword->Related())[1] == anvil &&
              anvil->Related() == nullptr && items.At(2).Related()->empty(), "an optional list of refs as a result");
    Check(sword->BestKey() == "anvil" && &sword->Best() == anvil && &items.At(2).Best() == sword, "a precomputed ref");
    Check(sword->PairFor(Tone::warning) == &items.At(2) && sword->PairFor(Tone::info) == nullptr &&
              *items.At(2).PairForKey(Tone::info) == "sword", "a lookup ref");
    Check(sword->BonusOr(7) == 7 && anvil->BonusOr(7) == -2, "?? on an optional field");
    Check(sword->OriginX(1) == -3 && anvil->OriginX(1) == 1, "?? through an optional record");
    Check(sword->NoteOr("x") == "x" && anvil->NoteOr("x") == "heavy", "?? on an optional String");
    const Reward& reward = sword->GetReward();
    Check(reward.GetKind() == RewardKind::item && reward.AsCoins() == nullptr, "variant kind");
    Check(reward.AsItem() != nullptr && reward.AsItem()->GetItemId() == "II_GEM" && reward.AsItem()->Total(3) == 15, "case class");
    Check(anvil->GetReward().AsCoins() != nullptr && anvil->GetReward().AsCoins()->GetAmount() == 9000000000ull, "UInt64");
    Check(items.At(2).GetReward().GetKind() == RewardKind::nothing, "fieldless case");
}

void CheckConfig(const Config& config) {
    Check(config.GetMotd() == "Welcome" && config.GetSpawn().GetY() == -20, "record value");
    Check(config.GetEvent().AsItem() != nullptr && config.GetEvent().AsItem()->GetItemId() == "II_KEY", "inline variant");
    Check(config.GetFlags().size() == 2 && config.GetFlags()[0] == Tone::warning, "list of enums");
    Check(config.GetScale() == 1.5, "optional Float");
    Check(config.GetFeaturedKey() == "sword" && config.GetFeatured().GetLabel() == "Sword", "a ref resolved in the snapshot");
    Check(config.GetPicksKeys().size() == 2 && config.GetPicks()[0]->GetLabel() == "Air", "refs resolved in the snapshot");
    Check(config.GetAlts() != nullptr && config.GetAlts()->size() == 1 && (*config.GetAlts())[0]->GetLabel() == "Anvil" &&
              config.GetAltsKeys() != nullptr, "an optional list of refs");
    const Node& tree = config.GetTree();
    Check(tree.GetNext() != nullptr && tree.GetNext()->GetLabel() == "second" && tree.GetNext()->GetNext() == nullptr &&
              tree.GetKids().size() == 1 && tree.GetKids()[0].GetLabel() == "leaf", "a record holding itself");
    Check(tree.GetHot() != nullptr && tree.GetHot()->GetLabel() == "Air" && tree.GetNext()->GetHot() == nullptr &&
              tree.GetKids()[0].GetHot()->GetLabel() == "Sword", "refs resolved through a boxed optional and a list");
    Check(tree.GetBadge() != nullptr && tree.GetBadge()->AsStar()->GetOf().GetLabel() == "Anvil" &&
              tree.GetKids()[0].GetBadge()->GetKind() == BadgeKind::plain, "a ref resolved in a variant case");
    Check(tree.NextLabel("x") == "second" && tree.GetNext()->NextLabel("x") == "x", "?? through a boxed optional");
}

void CheckShelves(const std::string& root) {
    std::string error;
    std::shared_ptr<const demo::shop::Shelves> shelves = demo::shop::Shelves::Load(root + "/good/shelves.json", error);
    Check(shelves != nullptr, "Shelves::Load: " + error);
    if (shelves != nullptr) {
        const Shelf* front = shelves->Find("front");
        Check(front != nullptr && front->GetSlots() == -1 && front->GetItemsKeys().size() == 2, "keyed list");
        Check(front != nullptr && front->GetNext() == shelves->Find("back") && shelves->Find("back")->GetNext() == nullptr,
              "a ref resolved in its own value");
        Check(shelves->FindByDelete(8) == shelves->Find("back") && shelves->FindByI(30) == front &&
                  shelves->FindByI(32) == nullptr, "FindBy with escaped and clashing names");
        if (front != nullptr) {
            Check(front->GetBonuses().size() == 2 && front->GetBonuses()[1].GetStat() == Tone::info &&
                      front->GetBonuses()[1].GetAmount() == 5 && shelves->Find("back")->GetBonuses().empty(), "pairs slots");
            Check(front->GetFlags().size() == 2 && front->GetFlags()[0] == Flag::a && front->GetFlags()[1] == Flag::c, "bits");
            Check(front->GetWaits().size() == 2 && front->GetWaits()[1].count() == 2000, "a unit on a list");
        }
    }
    error.clear();
    std::shared_ptr<const Point> home = Point::Load(root + "/good/home.json", error);
    Check(home != nullptr && home->GetX() == 5 && home->GetY() == -6, "a record value's static Load: " + error);
    error.clear();
    Check(demo::shop::Shelves::Load(root + "/bad/shelves.json", error) == nullptr, "a bad file is refused");
    std::printf("bad: %s\n", error.c_str());
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    const std::string root = argv[1];
    CheckStatics();
    std::string error;
    Check(ShopStore::Reload(root + "/good", error), "Reload(good): " + error);
    std::shared_ptr<const ShopSnapshot> snap = ShopStore::Current();
    if (snap != nullptr) {
        CheckItems(snap->GetItems());
        CheckConfig(snap->GetConfig());
    }
    CheckShelves(root);
    if (snap != nullptr) {
        error.clear();
        Check(!ShopStore::Reload(root + "/badsnap", error), "a ref naming no entry fails the Reload");
        std::printf("badsnap: %s\n", error.c_str());
        Check(ShopStore::Current() == snap && snap->GetConfig().GetFeatured().GetLabel() == "Sword",
              "the old snapshot and its resolved refs stay");
    }
    std::printf("failures: %d\n", failures);
    return failures;
}
