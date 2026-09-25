// Exercises runtime inputs end to end (CODEGEN.md §5.12, §7.7; EVALUATION.md §11.3): a getter
// before LoadInputs, missing variables, empty = unset, the happy path and re-reads, then argv[1]'s
// cases, one per line "<ENV> <hex of the value>", each read alone: prints the variable's failure
// line, or "ok".
#include "demo.gen.h"

#include <cstdio>
#include <cstdlib>
#include <fstream>
#include <string>
#include <string_view>

namespace {
std::string g_lastCode;
std::string g_lastMessage;
void Capture(std::string_view code, std::string_view message) {
    g_lastCode = std::string(code);
    g_lastMessage = std::string(message);
}
const char* const kVars[] = {"CANON_TEST_PORT", "CANON_TEST_DEBUG", "CANON_TEST_NAME",  "CANON_TEST_TAG",
                             "CANON_TEST_TIMEOUT", "CANON_TEST_RATE", "CANON_TEST_COLOR", "CANON_TEST_LEVEL",
                             "CANON_TEST_NOTE", "CANON_TEST_RATIO", "CANON_TEST_COUNT", "CANON_TEST_WAIT"};
void Baseline() {
    for (const char* n : kVars) unsetenv(n);
    setenv("CANON_TEST_PORT", "8080", 1);
    setenv("CANON_TEST_DEBUG", "true", 1);
}
// The line of err naming env, without its newline, or "ok".
std::string LineOf(const std::string& err, const std::string& env) {
    const size_t at = err.find(env + ": ");
    if (at == std::string::npos) return "ok";
    return err.substr(at, err.find('\n', at) - at);
}
std::string Unhex(const std::string& hex) {
    std::string out;
    for (size_t i = 0; i + 1 < hex.size(); i += 2) out.push_back(static_cast<char>(std::stoi(hex.substr(i, 2), nullptr, 16)));
    return out;
}
}  // namespace

int main(int argc, char** argv) {
    if (argc != 2) return 100;
    canon::SetEvalErrorHandler(&Capture);
    demo::Config cfg;

    // A getter before LoadInputs: E8302 with its full message, then the unset slot.
    const int32_t before = cfg.GetPort();
    std::printf("before: code=%s message=%s zero=%d\n", g_lastCode.c_str(), g_lastMessage.c_str(), before == 0);

    // Every required input missing: one line each; an empty variable is unset.
    for (const char* n : kVars) unsetenv(n);
    setenv("CANON_TEST_NAME", "", 1);
    std::string err;
    bool ok = demo::LoadInputs(err);
    std::printf("missing: ok=%d err=%s", ok, err.c_str());

    // Happy path, then a re-read, then a failed re-read that leaves the field unset.
    Baseline();
    setenv("CANON_TEST_NAME", "hi", 1);
    setenv("CANON_TEST_TAG", "ABC", 1);
    setenv("CANON_TEST_TIMEOUT", "5s", 1);
    setenv("CANON_TEST_RATE", "0.5", 1);
    setenv("CANON_TEST_COLOR", "red", 1);
    setenv("CANON_TEST_LEVEL", "mid", 1);
    ok = demo::LoadInputs(err);
    const auto timeout = cfg.GetTimeout();
    const auto rate = cfg.GetRate();
    std::printf("happy: ok=%d port=%d debug=%d name=%s tag=%s timeoutMs=%lld rate=%.2f color=%d level=%d\n", ok, cfg.GetPort(),
                cfg.GetDebug(), cfg.GetName() ? cfg.GetName()->c_str() : "none", cfg.GetTag() ? cfg.GetTag()->c_str() : "none",
                timeout ? static_cast<long long>(timeout->count()) : -1, rate ? static_cast<double>(*rate) : -1.0,
                cfg.GetColor() == demo::Color::red, cfg.GetLevel() == demo::Tier::mid);
    // LoadInputs replaces error, as every Load does (CODEGEN.md §5.12): a stale text is gone.
    setenv("CANON_TEST_PORT", "9090", 1);
    err = "stale";
    ok = demo::LoadInputs(err);
    std::printf("reread: ok=%d port=%d err=%s\n", ok, cfg.GetPort(), err.c_str());
    setenv("CANON_TEST_PORT", "x", 1);
    setenv("CANON_TEST_NAME", "", 1);
    ok = demo::LoadInputs(err);
    std::printf("failedreread: ok=%d port=%d name=%s\n", ok, cfg.GetPort(), cfg.GetName() ? "set" : "none");

    std::ifstream cases(argv[1]);
    std::string env, hex;
    while (cases >> env >> hex) {
        Baseline();
        const std::string value = Unhex(hex);
        setenv(env.c_str(), value.c_str(), 1);
        demo::LoadInputs(err);
        std::printf("%s %s %s\n", env.c_str(), hex.c_str(), LineOf(err, env).c_str());
    }
    return 0;
}
