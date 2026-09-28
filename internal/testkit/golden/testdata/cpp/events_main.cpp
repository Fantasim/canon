// Decodes each argument, a JSON file, with resource.events' types-mode EventConfig::Decode
// (CODEGEN.md §5.13; M3 acceptance 6). Prints one line per file: the decoder's error, or the
// events it decoded, with each monster's define value.
#include "events/events.gen.h"

#include <nlohmann/json.hpp>

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

namespace {

std::string Kind(const NMEvent::gen::EventKind& k) {
    if (const auto* m = k.AsSpawnMonster()) {
        const auto life = m->GetMonsterLifetime();
        return "spawn_monster " + m->GetMonsterIdKey() + "=" + std::to_string(m->GetMonsterIdValue()) +
               " lifetime=" + (life ? std::to_string(life->count()) : "none");
    }
    if (const auto* i = k.AsSpawnItem()) return "spawn_item " + i->GetItemIdKey();
    if (const auto* d = k.AsMonsterDropInject()) {
        return "monster_drop_inject " + d->GetItemIdKey() + " count=" + std::to_string(d->GetItemCount()[0]) + ".." +
               std::to_string(d->GetItemCount()[1]) + " levels=" + std::to_string(d->GetLevelMin()) + ".." +
               std::to_string(d->GetLevelMax());
    }
    return "?";
}

}  // namespace

int main(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        std::ifstream in(argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::string error;
        const auto config = NMEvent::gen::EventConfig::Decode(nlohmann::json::parse(text, nullptr, false), error);
        if (!config) {
            std::printf("error %s\n", error.c_str());
            continue;
        }
        std::string line = "version=" + std::to_string(config->GetVersion());
        for (const auto& e : config->GetEvents()) {
            line += " | " + e.GetId() + " " + Kind(e.GetKind()) + " windows=" + std::to_string(e.GetSchedule().size()) +
                    " roll=" + std::string(NMEvent::gen::ToWire(e.GetRollMode()));
        }
        std::printf("%s\n", line.c_str());
    }
    return 0;
}
