// TestReaderMatchesV020 and FuzzReaderMatchesV020 (DECISIONS 431): v0.2.0's reader and this compiler's give the
// same verdict, message, typed document, Float32 tokens and key orders on every text, as a ParseTextFile with key
// orders and without, and as a ParseDataFile with and without. The one listed difference: ParseDataFile refuses a
// NUL after the document, where v0.2.0's read the text up to it (WIRE.md §3.1).
//
//   reader_diff enumerate <seed>...  every prefix and deletion of each seed and, for a seed of at most 512 bytes,
//                                    every insertion and substitution of a byte of kBytes; one summary line per
//                                    setting, after at most kMaxShown DIFF lines
//   reader_diff serve                frames on stdin (a little-endian u32 length, then the bytes) until EOF; one
//                                    line per frame: "ok", or the first DIFF
#include <cstdint>
#include <cstdio>
#include <fstream>
#include <iterator>
#include <map>
#include <string>
#include <string_view>
#include <vector>

#include <nlohmann/json.hpp>

using Doc = nlohmann::json;
using Tokens = std::map<const Doc*, std::string>;
using Orders = std::map<const Doc*, std::vector<std::string>>;

// reader_oracle.cpp: v0.2.0's ParseTextFile and ParseDataFile.
bool OracleText(std::string_view text, Doc& doc, std::string& error, Tokens& tokens, Orders* orders);
bool OracleData(const std::string& text, std::string_view schema, Doc& doc, std::string& error, Tokens& tokens,
                Orders* orders);
// reader_subject.cpp: this compiler's.
bool SubjectText(std::string_view text, Doc& doc, std::string& error, Tokens& tokens, Orders* orders);
bool SubjectData(const std::string& text, std::string_view schema, Doc& doc, std::string& error, Tokens& tokens,
                 Orders* orders);

namespace {

constexpr std::string_view kSchema = "reader@1";
constexpr std::string_view kNulRefusal = "D: not a canon data file (invalid JSON)";
constexpr std::string_view kSchemaRefusal = "D: built from schema ";
const char* const kEnvelope[] = {"$fns", "$schema", "rows", "value"};  // keys whose letter case ParseDataFile checks
const char* const kUntouched = "untouched";                             // the document a read starts from
constexpr size_t kSmallSeed = 512;  // a seed this long or shorter also gets every insertion and substitution
constexpr size_t kShowBytes = 160;  // how much of an input a DIFF line shows
constexpr long kMaxShown = 20;      // DIFF lines enumerate prints at most

// The 41 bytes insertions and substitutions use: JSON's structure, whitespace, number and literal letters, and
// the edges of UTF-8's ranges.
const unsigned char kBytes[] = {0x00, 0x09, 0x0A, 0x0D, 0x20, '"',  '\\', '/',  ',',  ':',  '[',  ']',  '{',  '}',
                                '0',  '1',  '9',  '-',  '+',  '.',  'e',  'E',  'u',  'n',  't',  'f',  'a',  'd',
                                'D',  0x7F, 0x80, 0xBF, 0xC0, 0xC2, 0xE0, 0xED, 0xEF, 0xF0, 0xF4, 0xF5, 0xFF};

// What one read gave: its verdict, its message, and its document with every node's type, the tokens and the
// orders by RFC 6901 pointer.
struct Outcome {
    bool ok = false;
    std::string error;
    std::string detail;
    size_t tokens = 0, orders = 0;
};

struct Setting {
    const char* name;
    bool data;
    bool orders;
    long inputs, valid, tokens, ordered, nul, diffs;
};

Setting settings[] = {
    {"text+orders", false, true, 0, 0, 0, 0, 0, 0},
    {"text", false, false, 0, 0, 0, 0, 0, 0},
    {"data+orders", true, true, 0, 0, 0, 0, 0, 0},
    {"data", true, false, 0, 0, 0, 0, 0, 0},
};

long shown = 0;

std::string Escape(const std::string& key) {
    std::string out;
    for (const char c : key) {
        if (c == '~') {
            out += "~0";
        } else if (c == '/') {
            out += "~1";
        } else {
            out += c;
        }
    }
    return out;
}

// Every node of `v` by address, with its RFC 6901 pointer.
void Walk(const Doc& v, const std::string& at, std::map<const Doc*, std::string>& out) {
    out[&v] = at;
    if (v.is_object()) {
        for (auto it = v.begin(); it != v.end(); ++it) Walk(it.value(), at + "/" + Escape(it.key()), out);
    } else if (v.is_array()) {
        for (size_t i = 0; i < v.size(); ++i) Walk(v[i], at + "/" + std::to_string(i), out);
    }
}

// A string as its length then its bytes: never throws, whatever the bytes.
void Bytes(const std::string& s, std::string& out) { out += std::to_string(s.size()) + ':' + s; }

// `v` with the type of every node (operator== takes 1, 1u and 1.0 as equal); a float as `%a`.
void Typed(const Doc& v, std::string& out) {
    out += std::to_string(static_cast<int>(v.type())) + ':';
    if (v.is_object()) {
        out += '{';
        for (auto it = v.begin(); it != v.end(); ++it) {
            Bytes(it.key(), out);
            Typed(it.value(), out);
            out += ',';
        }
        out += '}';
    } else if (v.is_array()) {
        out += '[';
        for (const Doc& e : v) {
            Typed(e, out);
            out += ',';
        }
        out += ']';
    } else if (v.is_number_float()) {
        char b[64];
        std::snprintf(b, sizeof b, "%a", v.get<double>());
        out += b;
    } else if (v.is_string()) {
        Bytes(v.get_ref<const std::string&>(), out);
    } else {
        out += v.dump();
    }
}

// The node `p` names in `where`, or a mark when it is no node of the document.
std::string At(const std::map<const Doc*, std::string>& where, const Doc* p) {
    const auto it = where.find(p);
    return it == where.end() ? std::string("<foreign node>") : it->second;
}

Outcome Read(bool oracle, const Setting& s, const std::string& text) {
    Outcome o;
    Doc doc = kUntouched;  // a failed read leaves the document as it found it
    Tokens tokens;
    Orders orders;
    Orders* keep = s.orders ? &orders : nullptr;
    if (s.data) {
        o.ok = (oracle ? OracleData : SubjectData)(text, kSchema, doc, o.error, tokens, keep);
    } else {
        o.ok = (oracle ? OracleText : SubjectText)(text, doc, o.error, tokens, keep);
    }
    o.tokens = tokens.size();
    o.orders = orders.size();
    std::map<const Doc*, std::string> where;
    Walk(doc, "", where);
    Typed(doc, o.detail);
    std::map<std::string, std::string> byPointer;  // sorted by pointer, as nodes' addresses are not
    for (const auto& t : tokens) byPointer["t" + At(where, t.first)] = t.second;
    for (const auto& r : orders) {
        std::string keys;
        for (const std::string& k : r.second) Bytes(k, keys);
        byPointer["o" + At(where, r.first)] = keys;
    }
    for (const auto& e : byPointer) {
        o.detail += '\n';
        Bytes(e.first, o.detail);
        Bytes(e.second, o.detail);
    }
    return o;
}

std::string Show(const std::string& s) {
    std::string out;
    for (const unsigned char c : s.substr(0, kShowBytes)) {
        if (c >= 0x20 && c < 0x7F && c != '\\') {
            out += static_cast<char>(c);
        } else {
            char b[8];
            std::snprintf(b, sizeof b, "\\x%02X", c);
            out += b;
        }
    }
    return out + (s.size() > kShowBytes ? "..." : "");
}

bool EndsWith(const std::string& s, const std::string& end) {
    return s.size() >= end.size() && s.compare(s.size() - end.size(), end.size(), end) == 0;
}

// A finding of ParseDataFile's envelope, made after the byte rules passed: "D: built from schema …", or
// "D: <key>: differs from "<envelope key>" only in letter case".
bool Envelope(const std::string& error) {
    if (error.rfind(std::string(kSchemaRefusal), 0) == 0) return true;
    if (error.rfind("D: ", 0) != 0) return false;
    for (const char* key : kEnvelope) {
        if (EndsWith(error, std::string(": differs from \"") + key + "\" only in letter case")) return true;
    }
    return false;
}

// v0.2.0's ParseDataFile read a text up to its first NUL (nlohmann's end of input); this compiler's refuses a
// NUL after the document: where v0.2.0's passed the byte rules, the new reader says the text is not JSON and
// leaves the document untouched.
bool NulRefusal(const Setting& s, const std::string& text, const Outcome& old, const Outcome& now) {
    if (!s.data || text.find('\0') == std::string::npos || now.ok || now.error != kNulRefusal) return false;
    std::string untouched;
    Typed(Doc(kUntouched), untouched);
    return now.detail == untouched && (old.ok || Envelope(old.error));
}

// The first difference of the two readers on `text` under `s`, counted in its stats; empty when they agree.
std::string Differ(Setting& s, const std::string& text) {
    ++s.inputs;
    const Outcome old = Read(true, s, text);
    const Outcome now = Read(false, s, text);
    if (old.ok) {
        ++s.valid;
        if (old.tokens > 0) ++s.tokens;
        if (old.orders > 0) ++s.ordered;
    }
    if (old.ok == now.ok && old.error == now.error && old.detail == now.detail) return std::string();
    if (NulRefusal(s, text, old, now)) {
        ++s.nul;
        return std::string();
    }
    ++s.diffs;
    std::string what = old.ok != now.ok ? "verdict" : old.error != now.error ? "message" : "document";
    return "DIFF " + std::string(s.name) + " " + what + " [" + Show(text) + "] old ok=" + std::to_string(old.ok) +
           " err=[" + Show(old.error) + "] new ok=" + std::to_string(now.ok) + " err=[" + Show(now.error) + "]";
}

std::string DifferAll(const std::string& text) {
    for (Setting& s : settings) {
        std::string d = Differ(s, text);
        if (!d.empty()) return d;
    }
    return std::string();
}

void Compare(const std::string& text) {
    for (Setting& s : settings) {
        const std::string d = Differ(s, text);
        if (!d.empty() && shown++ < kMaxShown) std::printf("%s\n", d.c_str());
    }
}

void Enumerate(const std::string& seed) {
    Compare(seed);
    for (size_t i = 0; i <= seed.size(); ++i) {
        Compare(seed.substr(0, i));
        if (i < seed.size()) Compare(seed.substr(0, i) + seed.substr(i + 1));
        if (seed.size() > kSmallSeed) continue;
        for (const unsigned char c : kBytes) {
            Compare(seed.substr(0, i) + static_cast<char>(c) + seed.substr(i));
            if (i < seed.size() && static_cast<unsigned char>(seed[i]) != c) {
                std::string s = seed;
                s[i] = static_cast<char>(c);
                Compare(s);
            }
        }
    }
}

std::string ReadFile(const char* path) {
    std::ifstream in(path, std::ios::binary);
    return std::string(std::istreambuf_iterator<char>(in), std::istreambuf_iterator<char>());
}

// One frame from stdin into `text`; false at EOF or on a short frame.
bool Frame(std::string& text) {
    unsigned char n[4];
    if (std::fread(n, 1, sizeof n, stdin) != sizeof n) return false;
    const uint32_t size = static_cast<uint32_t>(n[0]) | static_cast<uint32_t>(n[1]) << 8 |
                          static_cast<uint32_t>(n[2]) << 16 | static_cast<uint32_t>(n[3]) << 24;
    text.assign(size, '\0');
    return size == 0 || std::fread(&text[0], 1, size, stdin) == size;
}

int Serve() {
    std::string text;
    while (Frame(text)) {
        const std::string d = DifferAll(text);
        std::printf("%s\n", d.empty() ? "ok" : d.c_str());
        std::fflush(stdout);
    }
    return 0;
}

}  // namespace

int main(int argc, char** argv) {
    if (argc < 2) return 2;
    const std::string mode = argv[1];
    if (mode == "serve") return Serve();
    if (mode != "enumerate") return 2;
    for (int i = 2; i < argc; ++i) Enumerate(ReadFile(argv[i]));
    for (const Setting& s : settings) {
        std::printf("%s: %ld inputs, %ld valid, %ld with Float32 tokens, %ld with key orders, %ld NUL refusals, %ld differences\n",
                    s.name, s.inputs, s.valid, s.tokens, s.ordered, s.nul, s.diffs);
    }
    return 0;
}
