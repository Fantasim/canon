// Decodes JSON files through the types-mode public decoders (CODEGEN.md §5.13): argv[1] is their
// directory, then one argument per file, "c:<file>" for an EventConfig and "x:<file>" for an
// Extras, "p:<file>" for a Picks, through Decode; the upper-case letter ("C:", "X:", "P:") reads the
// text through Parse instead (DECISIONS 361). Prints "== <argument>", then what each decoded, or the
// decoder's error.
#include "out/events/events.gen.h"

#include <nlohmann/json.hpp>

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

namespace {

using NMEvent::gen::Event;
using NMEvent::gen::EventConfig;
using NMEvent::gen::EventKind;
using NMEvent::gen::Extras;
using NMEvent::gen::Picks;

std::string ReadText(const std::string& path) {
    std::ifstream in(path, std::ios::binary);
    return std::string((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
}

// Decode of the parsed file, or Parse of its text.
template <typename T>
std::optional<T> Load(const std::string& path, bool parse, std::string& error) {
    const std::string text = ReadText(path);
    if (parse) return T::Parse(text, error);
    return T::Decode(nlohmann::json::parse(text, nullptr, /*allow_exceptions=*/false), error);
}

std::string Ms(std::optional<std::chrono::milliseconds> d) { return d ? std::to_string(d->count()) : "none"; }

std::string Region(const NMEvent::gen::Rect& r) {
    char text[128];
    std::snprintf(text, sizeof text, "%g,%g,%g,%g", r.GetLeft(), r.GetTop(), r.GetRight(), r.GetBottom());
    return text;
}

std::string Count(const std::vector<int64_t>& c) {
    std::string out;
    for (int64_t n : c) out += (out.empty() ? "" : "..") + std::to_string(n);
    return out;
}

void Kind(const EventKind& k) {
    if (const auto* m = k.AsSpawnMonster()) {
        std::printf("  spawn_monster %s=%lld region=%s lifetime=%s\n", m->GetMonsterIdKey().c_str(), static_cast<long long>(m->GetMonsterIdValue()), Region(m->GetSpawnRegion()).c_str(),
                    Ms(m->GetMonsterLifetime()).c_str());
    } else if (const auto* i = k.AsSpawnItem()) {
        std::printf("  spawn_item %s count=%s region=%s lifetime=%s\n", i->GetItemIdKey().c_str(), Count(i->GetItemCount()).c_str(),
                    Region(i->GetSpawnRegion()).c_str(), Ms(i->GetGroundLifetime()).c_str());
    } else if (const auto* d = k.AsMonsterDropInject()) {
        std::printf("  monster_drop_inject %s count=%s levels=%lld..%lld\n", d->GetItemIdKey().c_str(), Count(d->GetItemCount()).c_str(),
                    static_cast<long long>(d->GetLevelMin()), static_cast<long long>(d->GetLevelMax()));
    }
}

void Config(const std::string& dir, const std::string& name, bool parse) {
    std::string error;
    const std::optional<EventConfig> c = Load<EventConfig>(dir + "/" + name, parse, error);
    if (!c) {
        std::printf("%s: error %s\n", name.c_str(), error.c_str());
        return;
    }
    std::printf("%s: version=%lld events=%zu\n", name.c_str(), static_cast<long long>(c->GetVersion()), c->GetEvents().Len());
    for (const Event& e : c->GetEvents()) {
        std::printf(" %s world=%u target=%lld roll=%s\n", e.GetId().c_str(), static_cast<unsigned>(e.GetWorldId()),
                    static_cast<long long>(e.GetTargetCount()), std::string(ToWire(e.GetRollMode())).c_str());
        Kind(e.GetKind());
        for (const auto& w : e.GetSchedule()) {
            std::printf("  %s %02lld:%02lld-%02lld:%02lld\n", std::string(sov::time::ToWire(w.GetDay())).c_str(),
                        static_cast<long long>(w.GetStartUtc().GetHour()), static_cast<long long>(w.GetStartUtc().GetMinute()),
                        static_cast<long long>(w.GetEndUtc().GetHour()), static_cast<long long>(w.GetEndUtc().GetMinute()));
        }
    }
    const Event* found = c->GetEvents().Find("moonstone_rain");
    std::printf(" find moonstone_rain: %s\n", found != nullptr ? found->GetId().c_str() : "absent");
}

void Extra(const std::string& dir, const std::string& name, bool parse) {
    std::string error;
    const std::optional<Extras> x = Load<Extras>(dir + "/" + name, parse, error);
    if (!x) {
        std::printf("%s: error %s\n", name.c_str(), error.c_str());
        return;
    }
    const std::string* label = x->GetLabel();
    std::printf("%s: reqMp=%lld label=%s mode=%s delay=%lld ratio=%g\n", name.c_str(), static_cast<long long>(x->GetReqMp()),
                label != nullptr ? label->c_str() : "none", x->GetMode().c_str(), static_cast<long long>(x->GetDelay().count()), static_cast<double>(x->GetRatio()));
}

void Pick(const std::string& dir, const std::string& name, bool parse) {
    std::string error;
    const std::optional<Picks> p = Load<Picks>(dir + "/" + name, parse, error);
    if (!p) {
        std::printf("%s: error %s\n", name.c_str(), error.c_str());
        return;
    }
    std::string slots;
    for (const auto& s : p->GetSlots()) slots += " " + std::to_string(s.GetN()) + "=" + s.GetMode();
    std::printf("%s: a=%s slots=%s\n", name.c_str(), p->GetA().c_str(), slots.c_str());
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        const std::string arg = argv[i];
        const bool parse = arg.size() > 1 && arg[0] >= 'A' && arg[0] <= 'Z';
        const char kind = parse ? static_cast<char>(arg[0] - 'A' + 'a') : arg[0];
        if (arg.size() < 2 || arg[1] != ':') continue;
        std::printf("== %s\n", arg.c_str());
        if (kind == 'c') Config(dir, arg.substr(2), parse);
        if (kind == 'x') Extra(dir, arg.substr(2), parse);
        if (kind == 'p') Pick(dir, arg.substr(2), parse);
    }
    return 0;
}
