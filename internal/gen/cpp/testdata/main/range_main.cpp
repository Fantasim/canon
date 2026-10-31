// Decodes each argument file as a Probe (CODEGEN.md §5.15): argv[1] is the directory, then one case name per
// file "<name>.json". Prints "== D:<name>" and the line of Decode over the parsed JSON, then "== P:<name>" and
// the line of Parse over the text: "ok", or "error " and the decoder's message.
#include "range.gen.h"

#include <nlohmann/json.hpp>

#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>

namespace {

using demo::range::Probe;

std::string ReadText(const std::string& path) {
    std::ifstream in(path, std::ios::binary);
    return std::string((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
}

void Report(const std::optional<Probe>& probe, const std::string& error) {
    std::printf("%s\n", probe ? "ok" : ("error " + error).c_str());
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        if (argv[i][0] == '@') {  // a @text file: Decode<Fn>File of the list `caps`
            std::string error;
            std::printf("== T:%s\n", argv[i] + 1);
            const auto caps = demo::range::DecodeCapsFile(ReadText(dir + "/" + (argv[i] + 1) + ".json"), error);
            std::printf("%s\n", caps ? "ok" : ("error " + error).c_str());
            continue;
        }
        const std::string text = ReadText(dir + "/" + argv[i] + ".json");
        std::string error;
        std::printf("== D:%s\n", argv[i]);
        Report(Probe::Decode(nlohmann::json::parse(text, nullptr, /*allow_exceptions=*/false), error), error);
        error.clear();
        std::printf("== P:%s\n", argv[i]);
        Report(Probe::Parse(text, error), error);
    }
    return 0;
}
