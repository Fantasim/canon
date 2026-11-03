// Decodes JSON files through demo.parts' public Wide::Parse and prints every member it holds, or the error
// (CODEGEN.md §2.11, §7.6): argv[1] is the directory of the files, then one file name per argument. The
// same program runs against the package generated whole and cut into Decode<N> parts; the outputs must
// be equal, the first failure included.
#include "out/parts.gen.h"

#include <cstdint>
#include <cstdio>
#include <fstream>
#include <iterator>
#include <optional>
#include <string>
#include <vector>

namespace {

using demo::parts::Wide;

std::string Ints(const std::vector<int64_t>& v) {
    std::string out;
    for (int64_t n : v) out += std::to_string(n) + ",";
    return out;
}

std::string Opt(const std::optional<int64_t>& v) { return v ? std::to_string(*v) : "none"; }

void Print(const char* file, const Wide& w) {
    std::printf("%s:", file);
    std::printf(" i0=%lld", static_cast<long long>(w.GetI0()));
    std::printf(" i1=%lld", static_cast<long long>(w.GetI1()));
    std::printf(" i2=%lld", static_cast<long long>(w.GetI2()));
    std::printf(" i3=%lld", static_cast<long long>(w.GetI3()));
    std::printf(" i4=%lld", static_cast<long long>(w.GetI4()));
    std::printf(" i5=%lld", static_cast<long long>(w.GetI5()));
    std::printf(" i6=%lld", static_cast<long long>(w.GetI6()));
    std::printf(" i7=%lld", static_cast<long long>(w.GetI7()));
    std::printf(" i8=%lld", static_cast<long long>(w.GetI8()));
    std::printf(" i9=%lld", static_cast<long long>(w.GetI9()));
    std::printf(" i10=%lld", static_cast<long long>(w.GetI10()));
    std::printf(" i11=%lld", static_cast<long long>(w.GetI11()));
    std::printf(" i12=%lld", static_cast<long long>(w.GetI12()));
    std::printf(" i13=%lld", static_cast<long long>(w.GetI13()));
    std::printf(" i14=%lld", static_cast<long long>(w.GetI14()));
    std::printf(" i15=%lld", static_cast<long long>(w.GetI15()));
    std::printf(" s0=%s", w.GetS0().c_str());
    std::printf(" s1=%s", w.GetS1().c_str());
    std::printf(" s2=%s", w.GetS2().c_str());
    std::printf(" s3=%s", w.GetS3().c_str());
    std::printf(" s4=%s", w.GetS4().c_str());
    std::printf(" s5=%s", w.GetS5().c_str());
    std::printf(" s6=%s", w.GetS6().c_str());
    std::printf(" s7=%s", w.GetS7().c_str());
    std::printf(" b0=%d", w.GetB0() ? 1 : 0);
    std::printf(" b1=%d", w.GetB1() ? 1 : 0);
    std::printf(" b2=%d", w.GetB2() ? 1 : 0);
    std::printf(" b3=%d", w.GetB3() ? 1 : 0);
    std::printf(" f0=%g", w.GetF0());
    std::printf(" f1=%g", w.GetF1());
    std::printf(" f2=%g", w.GetF2());
    std::printf(" f3=%g", w.GetF3());
    std::printf(" o0=%s", Opt(w.GetO0()).c_str());
    std::printf(" o1=%s", Opt(w.GetO1()).c_str());
    std::printf(" o2=%s", Opt(w.GetO2()).c_str());
    std::printf(" o3=%s", Opt(w.GetO3()).c_str());
    std::printf(" l0=%s", Ints(w.GetL0()).c_str());
    std::printf(" l1=%s", Ints(w.GetL1()).c_str());
    std::printf(" at=%lld,%lld", static_cast<long long>(w.GetAt().GetX()), static_cast<long long>(w.GetAt().GetY()));
    if (const auto* d = w.GetShape().AsDot()) std::printf(" dot=%lld", static_cast<long long>(d->GetR()));
    if (const auto* b = w.GetShape().AsBlob()) {
        std::printf(" blob=");
        std::printf("%lld,", static_cast<long long>(b->GetP0()));
        std::printf("%lld,", static_cast<long long>(b->GetP1()));
        std::printf("%lld,", static_cast<long long>(b->GetP2()));
        std::printf("%lld,", static_cast<long long>(b->GetP3()));
        std::printf("%lld,", static_cast<long long>(b->GetP4()));
        std::printf("%lld,", static_cast<long long>(b->GetP5()));
        std::printf("%lld,", static_cast<long long>(b->GetP6()));
        std::printf("%lld,", static_cast<long long>(b->GetP7()));
        std::printf("%lld,", static_cast<long long>(b->GetP8()));
        std::printf("%lld,", static_cast<long long>(b->GetP9()));
        std::printf("%lld,", static_cast<long long>(b->GetP10()));
        std::printf("%lld,", static_cast<long long>(b->GetP11()));
        std::printf("%lld,", static_cast<long long>(b->GetP12()));
        std::printf("%lld,", static_cast<long long>(b->GetP13()));
        std::printf("%lld,", static_cast<long long>(b->GetP14()));
        std::printf("%lld,", static_cast<long long>(b->GetP15()));
        std::printf("%lld,", static_cast<long long>(b->GetP16()));
        std::printf("%lld,", static_cast<long long>(b->GetP17()));
        std::printf("%lld,", static_cast<long long>(b->GetP18()));
        std::printf("%lld,", static_cast<long long>(b->GetP19()));
        std::printf("%lld,", static_cast<long long>(b->GetP20()));
        std::printf("%lld,", static_cast<long long>(b->GetP21()));
        std::printf("%lld,", static_cast<long long>(b->GetP22()));
        std::printf("%lld,", static_cast<long long>(b->GetP23()));
        std::printf("%lld,", static_cast<long long>(b->GetP24()));
        std::printf("%lld,", static_cast<long long>(b->GetP25()));
        std::printf("%lld,", static_cast<long long>(b->GetP26()));
        std::printf("%lld,", static_cast<long long>(b->GetP27()));
        std::printf("%lld,", static_cast<long long>(b->GetP28()));
        std::printf("%lld,", static_cast<long long>(b->GetP29()));
        std::printf("%lld,", static_cast<long long>(b->GetP30()));
        std::printf("%lld,", static_cast<long long>(b->GetP31()));
        std::printf("%lld,", static_cast<long long>(b->GetP32()));
        std::printf("%lld,", static_cast<long long>(b->GetP33()));
        std::printf("%lld,", static_cast<long long>(b->GetP34()));
        std::printf("%lld,", static_cast<long long>(b->GetP35()));
        std::printf("%lld,", static_cast<long long>(b->GetP36()));
        std::printf("%lld,", static_cast<long long>(b->GetP37()));
        std::printf("%lld,", static_cast<long long>(b->GetP38()));
        std::printf("%lld,", static_cast<long long>(b->GetP39()));
        std::printf("%lld,", static_cast<long long>(b->GetP40()));
        std::printf("%lld,", static_cast<long long>(b->GetP41()));
        std::printf("%lld,", static_cast<long long>(b->GetP42()));
        std::printf("%lld,", static_cast<long long>(b->GetP43()));
        std::printf("%lld,", static_cast<long long>(b->GetP44()));
        std::printf("%lld,", static_cast<long long>(b->GetP45()));
        std::printf("%lld,", static_cast<long long>(b->GetP46()));
        std::printf("%lld,", static_cast<long long>(b->GetP47()));
        std::printf("%lld,", static_cast<long long>(b->GetP48()));
        std::printf("%lld,", static_cast<long long>(b->GetP49()));
        std::printf("%lld,", static_cast<long long>(b->GetP50()));
        std::printf("%lld,", static_cast<long long>(b->GetP51()));
        std::printf("%lld,", static_cast<long long>(b->GetP52()));
        std::printf("%lld,", static_cast<long long>(b->GetP53()));
        std::printf("%lld,", static_cast<long long>(b->GetP54()));
        std::printf("%lld,", static_cast<long long>(b->GetP55()));
        std::printf("%lld,", static_cast<long long>(b->GetP56()));
        std::printf("%lld,", static_cast<long long>(b->GetP57()));
        std::printf("%lld,", static_cast<long long>(b->GetP58()));
        std::printf("%lld,", static_cast<long long>(b->GetP59()));
        std::printf("%lld,", static_cast<long long>(b->GetP60()));
        std::printf("%lld,", static_cast<long long>(b->GetP61()));
        std::printf("%lld,", static_cast<long long>(b->GetP62()));
        std::printf("%lld,", static_cast<long long>(b->GetP63()));
    }
    std::printf("\n");
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 100;
    const std::string dir = argv[1];
    for (int i = 2; i < argc; ++i) {
        std::ifstream in(dir + "/" + argv[i], std::ios::binary);
        const std::string text((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
        std::string error;
        const std::optional<Wide> w = Wide::Parse(text, error);
        if (!w) {
            std::printf("%s: error %s\n", argv[i], error.c_str());
            continue;
        }
        Print(argv[i], *w);
    }
    return 0;
}
