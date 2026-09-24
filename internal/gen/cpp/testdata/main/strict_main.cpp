// Loads each argument, <kind>[:<locale>]=<path>, with the generated demo.shop and demo.nest
// loaders: "shelves" a file through Shelves::Load, "reload" a directory through
// ShopStore::Reload, "nested" a file through demo::nest::Holder::Load. A <locale> is set for the
// load only and must use ',' for the decimal point (the loaders ignore the process locale:
// log-2026-09-24 A1 C++ Float32). Prints one line per argument: the load error, or a canonical
// dump of the loaded Int/Float/Float32 fields (every one, optionals included), so the Go and C++
// loaders are compared on values, not only on refusals (log-2026-09-24 loader parity). The C++
// twin of strict_main.go.
#include "nest.gen.h"
#include "shop.gen.h"

#include <clocale>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <optional>
#include <string>

namespace {

// A signed, fixed-precision token distinguishing -0 from +0 ("+12.5", "-0"): the sign bit,
// then the magnitude at `precision` significant digits (log-2026-09-24 loader parity).
std::string SignedFloat(double v, int precision) {
    char buf[64];
    std::snprintf(buf, sizeof buf, "%s%.*g", std::signbit(v) ? "-" : "+", precision, std::fabs(v));
    return buf;
}

// "none" for an absent optional, else Int decimal or SignedFloat (log-2026-09-24 loader parity).
std::string OptInt(std::optional<int64_t> v) { return v ? std::to_string(*v) : "none"; }
std::string OptFloat(std::optional<double> v, int precision) { return v ? SignedFloat(*v, precision) : "none"; }

// A Float32's bits in hex ("3f800001"): exact, whatever the locale.
std::string Bits(float f) {
    uint32_t u = 0;
    std::memcpy(&u, &f, sizeof u);
    char buf[16];
    std::snprintf(buf, sizeof buf, "%08x", static_cast<unsigned>(u));
    return buf;
}

std::string DumpShelves(const demo::shop::Shelves& rows) {
    std::string out;
    for (size_t i = 0; i < rows.Len(); ++i) {
        const demo::shop::Shelf& r = rows.At(i);
        if (i > 0) out += ' ';
        out += r.GetId() + ":slots=" + std::to_string(r.GetSlots()) + ",delete=" + std::to_string(r.GetDelete()) +
               ",i=" + std::to_string(r.GetI());
    }
    return out;
}

std::string DumpItems(const demo::shop::Items& rows) {
    std::string out;
    for (size_t i = 0; i < rows.Len(); ++i) {
        const demo::shop::Item& r = rows.At(i);
        if (i > 0) out += ' ';
        out += r.GetId() + ":code=" + std::to_string(r.GetCode()) + ",price=" + SignedFloat(r.GetPrice(), 17) +
               ",weight=" + SignedFloat(r.GetWeight(), 9) + ",bonus=" + OptInt(r.GetBonus()) +
               ",legacyMax=" + std::to_string(r.GetLegacyMax());
    }
    return out;
}

std::string DumpConfig(const demo::shop::Config& c) { return "config:scale=" + OptFloat(c.GetScale(), 17); }

// Every Float32 of demo.nest's Holder, as Bits: f32s, "a.b", path a.b, shape's circle r, $scale.
std::string DumpHolder(const demo::nest::Holder& h) {
    std::string out = "f32s=";
    for (size_t i = 0; i < h.GetF32S().size(); ++i) {
        if (i > 0) out += ',';
        out += Bits(h.GetF32S()[i]);
    }
    const demo::nest::ShapeCircle* circle = h.GetShape().AsCircle();
    return out + " a.b=" + Bits(h.GetDotted()) + " a/b=" + Bits(h.GetDeep()) +
           " shape.r=" + (circle != nullptr ? Bits(circle->GetR()) : std::string("none")) +
           " $scale=" + Bits(h.Scale(demo::nest::Size::small)) + "," + Bits(h.Scale(demo::nest::Size::medium));
}

// Loads `path` as `kind`, then puts the "C" locale back before dumping: the dump or the error.
std::string Load(const std::string& kind, const std::string& path) {
    std::string error, out;
    if (kind == "shelves") {
        const auto s = demo::shop::Shelves::Load(path, error);
        std::setlocale(LC_ALL, "C");
        if (s) out = DumpShelves(*s);
    } else if (kind == "reload") {
        const bool ok = demo::shop::ShopStore::Reload(path, error);
        std::setlocale(LC_ALL, "C");
        if (ok) {
            const demo::shop::ShopSnapshot& snap = *demo::shop::ShopStore::Current();
            out = DumpItems(snap.GetItems()) + " " + DumpConfig(snap.GetConfig());
        }
    } else {
        const auto h = demo::nest::Holder::Load(path, error);
        std::setlocale(LC_ALL, "C");
        if (h) out = DumpHolder(*h);
    }
    return error.empty() ? out : error;
}

}  // namespace

int main(int argc, char** argv) {
    for (int i = 1; i < argc; ++i) {
        const std::string arg = argv[i];
        const size_t eq = arg.find('=');
        if (eq == std::string::npos) return 100;
        std::string kind = arg.substr(0, eq), locale;
        if (const size_t colon = kind.find(':'); colon != std::string::npos) {
            locale = kind.substr(colon + 1);
            kind.resize(colon);
        }
        if (kind != "shelves" && kind != "reload" && kind != "nested") return 100;
        if (!locale.empty() && (std::setlocale(LC_ALL, locale.c_str()) == nullptr ||
                                std::strcmp(std::localeconv()->decimal_point, ",") != 0)) {
            std::printf("cannot set a ',' locale %s\n", locale.c_str());
            continue;
        }
        std::printf("%s\n", Load(kind, arg.substr(eq + 1)).c_str());
    }
    return 0;
}
