- Linux: `canon_0.3.0_linux_amd64.tar.gz`, `canon_0.3.0_linux_arm64.tar.gz`
- macOS: `canon_0.3.0_darwin_arm64.tar.gz` (Apple silicon), `canon_0.3.0_darwin_amd64.tar.gz` (Intel)
- Windows: `canon_0.3.0_windows_amd64.zip` (`canon.exe`)

Verify with `checksums.txt` (`sha256sum -c checksums.txt`). On Linux and macOS,
`tools/install.sh` installs or updates canon.

## What changed

Lean generated code: smaller, faster generated C++, Go and TypeScript, plus a round of correctness fixes found while hardening (DECISIONS 430-516, ADR-0026). Compiler 0.3 reads **language 0.1**: every `canon: "0.1"` project is accepted unchanged (452).

- **Enum strings written once** in C++ and Go (430). On a large real project the generated `.gen.cpp` files shrank from 16.45 MB to 7.57 MB.
- **One-pass C++ JSON reader** (431), proven against 0.2.0's reader by a differential fuzz.
- **Baked containers** look keys up in sorted constant tables (432).
- **TypeScript** top-level values carry `/*#__PURE__*/` marks so a bundler can drop unused ones (433).
- **Generated functions stay within a line budget** (435): large decoders and lookups are split into parts.
- **C++ runtime copies carry a content stamp** (436), so two different runtime copies in one program fail at compile time instead of misbehaving.
- **C++ `Decode` accepts `nlohmann::ordered_json`** (437), and a variant's kind is a generated constant (438).
- **`out_templates`** (439): `project.canon` gives an emit's output path once, and an emit's `out` may name a root.
- **`canon lint`** (440): advisory lints for agents (`W0001`-`W0008`), never an error.
- **Shared values are walked once** in every evaluation stage and in the wire-form check (507, 509, 511, 513): deeply shared values no longer take exponential time.
- **Checker.** A target annotation on a local is `E1118` (443). A default reading a record literal's field is checked as that literal (444). Unstored dependent literals are judged (446). A layer file naming a broken declaration is broken, where it used to crash the compiler (514). A typo in a narrowing condition no longer cascades into presence errors, and hides none (516).
- **Identity.** An element assignment keeps its slot's key; another key is the new hard error `E4004` (512). A layer line keeps the key of the element its path names (`E1913`, 499).
- **Edit and workspace.** A long-lived Project no longer grows with its edits (445). About 30 correctness rulings for edits under layers, spreads, renames and `load.dir` tables (455-506), including: a refusal names a way out only when that way out works (462), and an edit whose Undo would be refused is refused before anything is written (474).
- **Build.** A build judges its outputs as inputs before they exist (`E8026` on the first run, 451). `E8005` hints name a way out that works (442).

## Upgrade notes (breaking)

- **Regenerate every C++ output with one canon.** A tree mixing outputs of two canon versions now stops at compile time with canon's `#error` (436).
- **`canon test` prints its warnings** and exits 4 past `--max-warnings` (449). **A test with an error outside its `expect`s now fails** (448).
- **A C++ data loader refuses a NUL byte after the document** (431).
- **New refusals** an existing project may meet: `E1118`, `E3204` at a record-literal default, `E3802` for unstored dependent literals, `E8026`, `E8019`, `E4004`, `E1913`.
- **New reserved generated names:** `E8005` covers the enum tables and the `Decode<N>`, `build<Last>_<N>` and `decode<T>_<N>` parts; `E8011` covers the new runtime identifiers.
- No Go or TypeScript API change.

## Verification

- `make check` green, with the C++ suites on g++, clang++ and clang++ with libc++.
- Long gates: `make stress`, `make fuzz-edit`, 31 fuzz targets, `make progen-nightly` (3 seeds at N=10000), `go test -race` at three GOMAXPROCS settings. Everything they found was fixed before the tag.
- Real project (142 packages, 795 tests): every command clean, 0 new findings. Every one of the 613,182 changed output lines is accounted for by a section of ADR-0026. The release gate compiles 506 generated units (118 C++, 282 Go, 106 TS): PASS. Downstream Go and TypeScript consumers type-check unchanged.
- Benchmarks against 0.2.0: no regression over 10%. Edit p95 is 0.395 s (target 0.450 s); language-server refresh p95 is 428 ms (cap 500 ms).
- Scale: `canon check` grows linearly. 15,000 entries take 5.6 s and 1.98 GB; 75,000 take 29.9 s and 9.85 GB.
- Costs: `canon test` takes about 25% longer and twice the memory, because it now runs the check pass too. Compiling the generated C++ costs about 10% more memory, from `ordered_json` support. Reusing the check pass's work in `canon test` is planned for 0.4.
- Not verified: Windows and macOS beyond `go vet` (no CI), and MSVC.
