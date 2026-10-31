# Sovereign -> Canon: the web-client direction -- what Canon would need, and what the generated code costs today

**From:** Sovereign Source (Louis, exploration session 2026-10-31). **Canon version:** 0.1.13.
**Measured at:** Source `test` @ `960ecb5cd`, Resource @ `df8dc6a4`, services as checked out that day.

**Priority: NOT SCHEDULED -- exploration.** Louis is exploring a direction, not planning it ("right now I'm just
talking with you, not planning anything, I just want to explore possibilities"). Nothing here is an accepted
Sovereign decision and no Sovereign ADR covers it yet. Do not start any item until Louis schedules it.

**How to read this.** It is written from the *Source* side: what the consumer would need, with the numbers
behind each need. It is deliberately long so the Canon side can judge for itself. Sovereign proposes **needs**,
not Canon designs:

- Any syntax or shape sketched here is illustrative only.
- Where a need collides with a Canon law, a DECISIONS entry or the refused list, say so and propose the
  Canon-shaped alternative. Overriding the law is not the expected answer.
- Where something should not be Canon's job at all, say that too. Several items below are marked
  "maybe not Canon" on purpose.

It extends two earlier asks, which stay valid and are not repeated in full:

- `2026-10-29-sovereign-cpp-macro-collisions.md`
- `2026-10-30-sovereign-cpp-checked-widths.md` (refined bounds -> checked decode and sized members; its
  "generated size" section is superseded by §6 below, which has fuller numbers).

**Provenance.** Most facts come from two read-only audits run for this session:

1. A capability audit. It read all 14 `canon guide` topics and wrote throwaway `canon check` probes in a
   scratch project; `canon build` was never run.
2. A generated-code audit. Static measurement of the C++/Go/TS outputs with `wc`, `md5sum` and scripts.

**[M]** = measured. **[I]** = inferred, not verified. Error codes quoted are what the 0.1.13 probes printed.

---

## 1. The direction in one page

Louis wants a **browser client** for the game, next to the existing C++ Windows client (`Neuz.exe`, DirectX 9),
which stays alive as long as possible, so players can choose.

| Piece | Choice being explored |
|---|---|
| Renderer | **Babylon Lite** (`@babylonjs/lite`): the Babylon team's WebGPU-only engine, no WebGL fallback, small bundle |
| UI | the client's windows rewritten in **HTML/CSS/JS** (TS) |
| Language | **TypeScript** for everything the browser presents; numeric kernels in C++ compiled to WebAssembly where speed or bit-exactness matters |
| Transport | **WebSocket** (browsers cannot open raw TCP) on the edge servers. **Session resume** (token, seq/ack, replay buffer, grace period) so link drops survive, for both clients |
| World | a **new streamable world format** replacing the legacy `.wld/.lnd/.dyo/.rgn` files: chunked, content-addressed, read by the WorldServer, the C++ client and the browser alike. Louis: "I don't feel like keeping the dyo, lnd etc. is the best way as it's legacy". He wants the new format for gameplay possibilities first, performance second, and accepts reworking the C++ client for it |
| Effects | **in-house SFX/particle system** replacing Effekseer (2,776 `.efkefc` + 242 `.efkmodel` today [M]). Effect definitions become data, so both renderers read one definition |
| Code sharing | **Canon becomes the main generator.** The rule Louis set: keep "the strict necessary" hand-translated between C++ and JS. Hand-written code in two languages is a bug source to be designed out |

### 1.1 The sharing model (one mechanism per layer)

| Layer | What | Mechanism | Hand-written twice? |
|---|---|---|---|
| L1 | static data: item/skill/mover/ctrl props, text, UI ids, vocab | Canon emits C++ + TS (+ Go for services) | no |
| L2 | formulas derived from data: tooltip stats, upgrade/rarity/Azure multipliers, costs | Canon `export fn`, translated per target with conformance vectors (Sovereign ADR L-0148 is the first planned use) | no |
| L3 | wire protocol between client and edge servers | a schema plus generated codecs: C++ (byte-compatible with the existing archive during migration) + TS (`DataView`) | no -- the biggest piece |
| L3b | runtime state shapes: an item instance, public mover state, buffs | Canon `types` with encoders and builders | no |
| L4 | hot numeric rules: movement integration, terrain height sampling, collision, CombatMath | a pure C++ library (no globals) built native and as WebAssembly. **Probably not Canon**; Canon may own its constants and golden test vectors | no |
| L5 | world/asset streaming, SFX definitions | Canon owns definitions and metadata (worlds, zones, spawns, placements, chunk index, effect definitions); a **baker** (not Canon) owns binary geometry, textures and content hashes | no |
| -- | presentation: Babylon scene, HTML UI, input; DX9 scene and CWnd UI | per client | **yes, unavoidably** |

Design rule on the Source side: **make the client dumber.** Every value the server sends already computed is
one less rule to share. That shrinks L2/L4. It does not remove L1/L3, which are mandatory.

Endgame Louis may take later: the desktop client becomes the web client in a native shell (Tauri/WebView2), and
the C++ client retires. Until then every presentation feature exists twice, which is why everything below
presentation must be generated.

---

## 2. Source-side facts Canon should know

### 2.1 The protocol today (L3's starting point)

- **Transport:** standalone Asio, `Source/Engine/Net/Transport/`.
  - Frame: `0x5E '^'`, then a uint32 little-endian payload length, then the payload. Max frame 1 MiB.
  - The payload can be wrapped in an LZ4 envelope (`0xFF 'L'`, 6-byte header, used above 64 bytes).
  - On the client-to-Cache link, an AES-256-CTR + HMAC session cipher (ECDH P-256 handshake) applies after LZ4.
  - [M] `Engine/Net/Transport`, `Engine/Net/Cipher`.
- **Topology:** the client talks to LoginServer (port 23000: certify, server list, characters) and then to
  CacheServer (5400). Cache proxies to the WorldServers. The client never talks to a WorldServer directly [M].
  So the WebSocket listener and session resume both live at the edge: no Canon impact, context only.
- **Packet ids:** `Source/Engine/Net/Packets/packet_ids.tsv` is rendered into `PacketIds.gen.h` by a Python
  generator (`Tools/build/gen_packet_ids.py`).
  - **862 `PACKETTYPE_*` + 471 `SNAPSHOTTYPE_*`** ids; 220 rows are guarded by a feature flag (`#ifdef`).
  - Direction is mostly not recorded [M].
  - Release builds permute ids (`PacketIds.release.h`, an anti-bot measure). Only 15 ids are "stable" across
    releases [M].
- **Serialization:** every packet is hand-written with the `CAr` archive (`Engine/Net/Misc/Archive/ar.h`):
  - `ar << x` / `ar >> x`;
  - little-endian fixed widths;
  - length-prefixed strings with a capacity bound on read;
  - no schema and no per-packet version marker [M].
  - The id enum is the only generated wire artifact.
- **Handler counts** [M]:
  - client `Client/Neuz/DPClient.cpp`: 66 `ON_MSG` + ~318 `case SNAPSHOTTYPE_`;
  - server `Server/World/Network/DPSrvr.cpp`: 282 `ON_MSG`.
  - So the client-facing surface is a few hundred messages, not all 1,333.
- **Snapshots:** the server batches many small typed records into one `PACKETTYPE_SNAPSHOT` per user per tick:
  `CUser::Add*` writes into `m_Snapshot`; ~327 distinct snapshot ids are referenced [M]. A Canon wire model must
  cover "a stream of tagged records in one frame", not only "one message per frame".
- **Versioning law (Sovereign DOCTRINE §0 rule 2):** any wire-format change must be gated on the client version
  string `__NEUZ_MSGVR`. An ungated breaking change is forbidden. Generated codecs must make that gate structural.
- **Existing headless client:** `Source/Apps/BotClient` (Lua-scripted, 59 files) already runs the certify ->
  Cache -> cipher -> LZ4 sequence without rendering. The first web experiment would be a TS twin of it on
  generated codecs. That is the acceptance vehicle for a Canon wire target.

### 2.2 Entity state on the wire (L3b)

- `CMover::Serialize` lives in `Engine/Serialization/ObjSerializeOpt.cpp` (1,022 lines). It branches on a
  "serialize way" enum `{TO_CLIENT, TO_WORLD, FROM_WORLD, FROM_DB, TO_DB}` plus `#ifdef`s [M]. **One shape, five
  projections**: the same state is written differently per audience. A Canon state model should express
  projections (which fields go to the owning client, other clients, DB, inter-server), or Sovereign keeps
  several records.
- `Engine/Serialization/NMSerialization.cpp` (1,280 lines): public and private player extension blocks [M].
- `CItemElem::Serialize` (`Engine/Serialization/ObjSerialize.cpp:26`) is **one function shared** by client,
  world and DB [M]. It is the first candidate for a generated state codec.
- A second item codec already exists: `ItemSerializer.cpp` (varint, tagged fields, Base62), mirrored by the
  Canon package `items/blob`.
  - That package already hand-rolls `pow2` because the language has no bit operators (`blob.canon:100`) [M].
  - This is the in-tree evidence that a varint/tagged binary form is needed.

### 2.3 Numeric semantics Canon must match

- **Single precision.** FlyFF math is `float`. L-0148 notes the tooltip formulas are single-precision, e.g.
  `floor(upgrade^1.2)`. `canon_runtime.h:60` says "Int is int64_t, Float is double", and `Float32` is storage
  only [M]. A translated formula computed in double gives silently different results at the edges.
- **int64 money.** Sovereign ADR L-0125 made penya (gold) int64. TS `Int` is `number`, and values past 2^53
  throw (`E8303`) [M].
- **Golden values come from legacy C++.** The translated-fn conformance proves target == Canon's evaluator. For a
  formula ported *from* existing C++, Sovereign also needs target == legacy C++ on captured inputs. Sovereign can
  capture the vectors. The question is whether Canon can host them as package tests.

### 2.4 Shared code today (why generated beats hand-ported)

- **359 translation units are compiled into both Neuz and WorldServer** (Neuz 1,008 TUs, WorldServer 580) [M]:
  movers, the action FSM, CombatMath, world, serialization, 72 Canon-generated TUs.
- `#if...__CLIENT` lines: Engine 838, Game 685. `__WORLDSERVER` lines: Engine 1,226, Game 1,310 [M].
- The client computes some rules locally:
  - movement is client-driven (the server validates and corrects);
  - max HP goes through CombatMath;
  - tooltip stats are built in `CWndMgr::MakeToolTipText` with 48 `Put*` helpers.
- **The tooltip content is already built 4-5 times** (client C++ plus three Go ports: admin, website,
  monitoring; plus a partial wiki one), and the ports drift. That is L-0148's motivation, and a preview of what
  happens to every rule if a browser client is hand-ported.

### 2.5 Assets and world

- Resource on disk [M]:
  - .o3d 14,254, .ani 12,888, .chr 1,900, .dds 15,629 (models);
  - .lnd 1,766, .wld/.rgn/.dyo 45 each (world);
  - .efkefc 2,776 (Effekseer);
  - UI .tga 958; icons png 5,993 / dds 5,315; audio wav 1,676 / ogg 51.
- **The WorldServer also loads world files**: heights, statics, an md5 anti-cheat map key, spawns. It also
  compiles the model loaders for bounds, skeletons and motions [M].
- **Server heights disagree with the client's at tile edges in 53% of 3,103 shared edges** (Sovereign ADR L-0120
  context) [M per that ADR]. One world format read by everyone removes that class.
- A model converter (o3d/chr/ani -> glb) exists outside the repos (Python, `~/Desktop/Flyff/convertor`).
- Sovereign ADR L-0120 (world/terrain refactor, executed by Anthony) is a refactor *of* the legacy formats. The
  new-format direction would supersede part of it. That is a Sovereign-side coordination point, not Canon's.

---

## 3. What Canon would own: a sketch of the package families

Illustrative only. Names and shapes are Source's guess; the Canon side should redesign freely.

| Family | Holds | Emits | Notes |
|---|---|---|---|
| `proto.*` | message records, direction (client->server, server->client), message ids (stable), protocol version, snapshot record types | C++ codec, TS codec, (Go for test tooling) | replaces `packet_ids.tsv` + hand `CAr` code, migrated one message at a time |
| `state.*` | runtime state records: `ItemInstance`, `MoverPublic`, `MoverPrivate`, `BuffState`... with audience projections | C++ / TS types + encoder/decoder + builders | `CItemElem` keeps behaviour by hand and *holds* a generated state member |
| `rules.*` | data-derived formulas | translated `export fn` | L-0148 phase 2 is the first user |
| `fx.*` | effect definitions: emitters, curves, gradients, ribbons, mesh particles, timing | data (binary target, §5.3) + types for both renderers | converted once from Effekseer by a Sovereign tool. Scale: ~2,800 effects, each with keyframe arrays |
| `world.*` | world list, zones, regions, spawn tables, gameplay layers, object placements by asset id, chunk index (logical) | data + types | metadata only. Heights/meshes/textures are the baker's. Placement counts not yet censused (Sovereign owes that number) |
| existing data packages | items, skills, movers, text... | + TS emits, binary data target | L1 |

What Source assumes stays **outside** Canon (confirm or correct):

- binary geometry, heightfields, textures, audio;
- content hashing of those files and the CDN manifest (see §7.1);
- the L4 numeric kernel (C++ -> WebAssembly);
- presentation code.

---

## 4. Capability gaps, by layer

Status as found in 0.1.13. MISSING = not expressible today. PARTIAL = expressible for JSON, or for some targets
only. Already-available features are left out unless needed for context.

### 4.1 L1 -- static data to C++ + TS

| Need | Status | Evidence / detail |
|---|---|---|
| Big tables in TS without one giant module | PARTIAL | TS `baked` puts every row in one `Object.freeze` module (`items.vocab.core.ts` is 75 KB from vocab alone) [M]. `data` mode is accepted for ts by `canon check` (probe) but no package uses `data` in any target [M] |
| Per-consumer selection finer than a whole `let` | PARTIAL | `values` selects whole `let`s; one emit per target per package (`E8002`), so a server and a browser cannot get different shapes of the same package in the same language |
| Binary data instead of JSON (server boot and browser fetch) | MISSING | outputs are code, JSON, text or view. See §6 finding 3 for the cost of JSON today |
| Game text in TS | PARTIAL | `text/client/emit.canon` emits text/cpp/go, no ts. A browser would fetch `textClient.json` with no generated decoder wired |
| Runtime inputs in a TS package | MISSING | layers: "A TypeScript emit refuses a package with inputs (`E8104`)" |

### 4.2 L2 -- formulas (`export fn` translated subset)

| Need | Status | Evidence / detail |
|---|---|---|
| Record, list or optional runtime parameter (an item instance passed at runtime) | MISSING | probe `E9006` "translated functions take Bool, integers, Float, String, Duration and enums"; `E9003` "parameter x ... is optional" |
| Read a table/map at runtime (multiplier rows indexed by a runtime level) | MISSING | probe: "`tiers.get(k) ?? 0` is not allowed ... outside the portable subset". This is L-0148's open question 2 |
| `pow`, `sqrt` | MISSING | probe `E9001` on both; allowed list "`floor`, `ceil`, `round`, `clamp`". L-0148 needs `floor(upgrade^1.2)` |
| `match`, `var`, bounded loops | MISSING | probe `E9001` on `match k`, `var`, `while` |
| Bitwise `& \| ^ << >>` | MISSING | probe `E1106` "unexpected character &". Needed for flag bytes, awake-slot packing, bitmask fields |
| Float32 arithmetic (C++ `float`, TS `Math.fround` at every step) | MISSING | `canon_runtime.h:60` "Float is double"; `Float32` is storage only (`float GetLeft()`) |
| Format a Float in templates ("12.5%") | MISSING | probe `E9005` "may only interpolate String, integer and enum values" |
| int64 in TS (bigint or hi/lo) | PARTIAL | `canonInt`: "integer outside the TypeScript safe range" (`E8303`) throws past 2^53. No `bigint` path. Division/modulo go through `BigInt` per operation [M], slow [I] |
| Conformance against legacy C++ golden values | PARTIAL [I] | vectors prove target == Canon evaluator. Whether hand-captured vectors can be package tests over translated fns at scale is unclear. A single vector is capped at 1,000,000 steps (`E9009`, probe) |
| Large finite-input fn | PARTIAL | finite lookup `export fn` (Bool/enum/ref params) becomes a full table, "at most 65 536 cells, `E9002`". See §6 finding 10: a 14-Bool fn became a 292 KB TS literal |

### 4.3 L3 -- wire protocol

| Need | Status | Evidence / detail |
|---|---|---|
| Binary codec target: **encode and decode**, C++ and TS | MISSING | the `wire` guide is "`@json` forms in depth". No guide mentions binary, bytes or varint. `types` emits "a decoder per public record", JSON only |
| C++ byte layout compatible with `CAr` during migration | MISSING | little-endian fixed widths, length-prefixed strings (uint32 length prefix [I -- Source to confirm per overload]), bounded reads. Needed so packets move one at a time with the legacy C++ on the other end |
| Record evolution: field present from protocol version N; message added or retired | MISSING | `past`/`retired` cover enums, refs and entries only. wire: "data runtimes refuse older files until rebuilt". Sovereign needs the reverse: an old peer and a new peer agreeing on version N |
| Protocol version constant generated (feeds `__NEUZ_MSGVR`) | MISSING [I] | a `const` can carry it. What's missing is tying field presence to it |
| Message id registry with permanent ids | HAVE (shape) | `stable table` + `code: UInt16 @stable` + `canon.lock`. Untested: the flag-guarded rows and the release permutation (§7.4) |
| Tagged unions in binary | PARTIAL | `variant @json(tag:)` / `@json(inline)`, JSON only |
| Bitmask fields in binary | PARTIAL | `@json(bits)`, JSON only, and no bit operators in the language |
| Varints, bounded arrays/strings in binary | MISSING | `items/blob` names `WIRE_VARINT` only as a const |
| A stream of tagged records in one frame (snapshots) | MISSING | see §2.1 |
| Audience projections of one state record | MISSING [I] | see §2.2. Could be several records plus a shared base if Canon prefers that |

### 4.4 L3b -- runtime state types

| Need | Status | Evidence / detail |
|---|---|---|
| Build or mutate an instance at runtime in C++/TS | MISSING | generated C++ classes have getters only. emit: "make hooks ... are for generated code, not API" |
| Encoder for a `types` record | MISSING | decode only |
| Sized members from refined ranges | asked | `2026-10-30-sovereign-cpp-checked-widths.md` |
| Open ids in C++/TS (older binary reads newer data) | PARTIAL | README: "`open:` on a Go `types` emit". Go only, refused for baked/data |

### 4.5 L4 -- numeric kernel

| Need | Status | Evidence / detail |
|---|---|---|
| Own the kernel's constants | HAVE | `const` -> `inline constexpr` |
| Own golden test vectors for a C++/WASM kernel | PARTIAL [I] | could be a `[Record]` `let` of input/expected rows emitted to C++/TS, if row counts in the thousands are fine for the budget |
| Generate the kernel itself | out of scope | Laws: generated code holds "types, values, lookups and getters, plus translated `export fn`". Source does not ask for it |

### 4.6 L5 -- world, assets, SFX

| Need | Status | Evidence / detail |
|---|---|---|
| Asset refs with existence checks | HAVE | assets: "the file must exist, exact case (`E3701`)" |
| Asset content hash / size in data (manifest) | MISSING, maybe not Canon | an asset is "a plain `String` on the wire". Purity law forbids no file read *under roots* [I], but hashing gigabytes per check is a cost question (§7.1) |
| Split one table into per-entry / per-chunk output files | MISSING | `emit json` dir is "`<value>.json` each" (per `let`); `@text` needs one literal name per fn |
| Content-hashed output names + generated manifest of Canon's own outputs | MISSING | `data` loader "checks its fingerprint" (type schema, not content). `canon.outputs` has hashes for ownership only |
| Effect definitions as data at ~2,800-effect scale | HAVE in kind, budget unknown [I] | records/variants/lists fit. Keyframe arrays x 2,800 effects may stress `E4401` and check time. Package split per effect family would spread the per-package budget |
| Spatial checks at build time (overlapping regions, spawns inside bounds) | HAVE, budget risk | build-time loops exist; budget is per package |

---

## 5. Missing features ranked (Source's view of "needed for sure")

1. **Binary wire target** (C++ + TS, encode + decode, CAr-compatible mode for migration). L3 rests on it.
2. **Record/message versioning tied to a protocol version** (field `since N`, message retired at N). DOCTRINE's
   `__NEUZ_MSGVR` gate made structural.
3. **Builders + encoders for `types` records** (runtime state must be constructed and sent, not just decoded).
4. **Float32 arithmetic semantics** in translated fns (`float` in C++, `Math.fround` in TS, a defined rounding
   for `floor`/`round` on float32).
5. **`pow` / `sqrt`** in the translated subset (or a curated math set with defined float32/float64 results per
   target).
6. **Runtime record/list/optional parameters and runtime table lookup** in translated fns.
7. **Bitwise operators.**
8. **Binary data target** shared by C++ (memory-mapped, zero-copy views) and TS (`ArrayBuffer`). Doubles as the
   server boot fix (§6 finding 3).
9. **Per-entry/per-chunk output splitting + content-hashed names + manifest** of Canon's own outputs.
10. **int64 in TS** (`bigint` or a hi/lo type) where a range passes 2^53.
11. `match` + bounded loops in the subset, Float formatting in templates, game text emitted to TS, runtime
    inputs in TS packages.

---

## 6. How the code is generated today -- measured costs

This is the half Louis asked about explicitly: "since we move to generated code ... optimization in the way the
code is generated, to avoid duplication, better performance". The numbers are from the 0.1.13 outputs as
committed in Source and as generated in the services.

### Finding 1 -- volume [M]

- `Source/Engine/Resource/Generated/`: 296 files, **739,551 lines, 29.4 MB**. Plus:
  - `Engine/Telemetry/Generated` (5 files, 0.70 MB);
  - `Tools/telemetry/generated` (6 files, 1.09 MB);
  - `Engine/Service/Notif/Generated` (2 files, 9 KB).
- By mode:
  - **baked** 84 files / 595 k lines / 23.7 MB (81%, almost all vocab enums);
  - **types** 162 files / 108 k lines / 4.3 MB;
  - runtime copies 53 files / 1.9 MB;
  - **`data` mode is used by no package**.
- Emit census: cpp 36 baked / 80 types; go 39 baked / 43 types; ts 35 baked / 1 types; text 85; view 111.
- Largest:

  | File | Size |
  |---|---|
  | `text/vocab.gen.cpp` / `.h` | 4.55 / 3.22 MB |
  | `items/vocab.gen.cpp` / `.h` | 2.64 / 1.86 MB |
  | `header/ids/ids.gen.cpp` | 2.09 MB |
  | `ai/vocab.gen.cpp` / `.h` | 1.41 / 0.97 MB |
  | `character/vocab.gen.cpp` | 0.94 MB |
  | `ui/vocab.gen.cpp` | 0.80 MB |
  | `text/vocab.defines.gen.h` | 0.49 MB |
  | `ui/neuz.gen.cpp` / `.h` | 0.39 / 0.34 MB |

- **Share per executable.** Unique generated .cpp in each exe's source list vs that exe's direct hand-written
  list:

  | Exe | Generated TUs | Generated lines | Hand-written lines |
  |---|---|---|---|
  | WorldServer | 79 | 395 k (15 MB) | 311 k |
  | Neuz | 95 | 388 k | 681 k |
  | CoreServer | 46 | 343 k | 68 k |
  | DatabaseServer | 43 | 336 k | 87 k |

- At 5-10x (the direction above) generated code becomes 80-95% of every server's direct sources [I].

### Finding 2 -- macro shims and the guards they force [M]

- The 12 `*.defines.gen.h` shims (~31 k `#define`s, the legacy `TID_*`/`II_*`/... names) collide with enum
  member names. So every generated header and .cpp wraps names in `#pragma push_macro` / `#undef` /
  `pop_macro`:
  - **3.92 MB of the 10.3 MB in `.gen.h` (38%)**;
  - **3.91 MB of the 16.05 MB in `.gen.cpp` (24%)**;
  - about **7.8 MB, 27% of all generated C++**.
- In `text/vocab.gen.h`, 49 k of 74 k lines are guards.
- The shims are force-fed into the precompiled headers: Neuz 9, WorldServer 4, CoreServer 3, DatabaseServer 5.
  `text/vocab.defines.gen.h` is included by 445 files.
- Related open ask: `2026-10-29-sovereign-cpp-macro-collisions.md`.
- Source-side note: the macros exist because ~10^4 legacy call sites use `TID_FOO` etc. Retiring them is a
  Sovereign sweep. Canon's part would be a non-macro spelling that the sweep can target (namespaced `constexpr`
  or the enum), plus dropping the guards once no shim is emitted.

### Finding 3 -- loader pipeline and data format [M]

- Every C++ data package is `types` mode. `ParseTextFile` does **five passes per file**: `SyntaxScan`,
  `TextScan`, SAX `StrictCheck`, a full nlohmann DOM parse, the `Decode` walk. A Sovereign `*FromCanon` copy
  follows (being removed, see finding 5).
- Per field: two DOM lookups (`v.contains` and `dec.Optional`) plus a compare against the `"="` string. Enum
  fields resolve by binary search over strings (`FromWire`).
- `Resource/Server` holds 138 JSON files, **43.3 MB**:

  | File | Pretty | Compact | Note |
  |---|---|---|---|
  | `propItem.json` | 25.2 MB | 17.7 MB | 7,252 rows x 127 keys; **708,536 cells (77%) are the `"="` none marker** |
  | `mdlDyna.json` | 6.1 MB | 3.1 MB | |
  | `skills.json` | 0.93 MB | 0.42 MB | |
  | `EventConfig.json` | 0.90 MB | 0.35 MB | |

  Then `serverNames` 1.5 MB, `mdlObj` 1.4 MB, `propMover` 1.2 MB, `Ids/text` 0.46 MB, `itemBoxes` 0.39 MB,
  `MonsterDrops` 0.37 MB.
- A DOM of ~920 k values for `propItem` is likely >100 MB transient [I].
- `propItem.json` is still parsed by a hand nlohmann loader today. Sovereign L-0147 wave E moves it onto the
  generated `Item` decoder, so this pipeline's cost lands on the biggest file soon.
- The browser would download the same data. A JSON form at this size is wrong for both.

### Finding 4 -- record layout [M]

- `sov::items::Item` has 69 `optional<int32_t>`, 43 `optional<enum>`, 6 `optional<string>` and 4
  `optional<double>` fields. Estimate: **~1.1-1.2 KB per row** on MSVC [I]; ~0.4 KB with presence bits and range
  widths [I].
- Tree-wide: 1,086 `int64_t` and 434 `double` fields against 323 32-bit integer fields. 761 numeric fields are
  `std::optional`, plus 269 `optional<string>`.
- Getters are fine: strings by const reference, optionals by value.
- **Why this is urgent:** Sovereign L-0147 (rulings 2026-10-30): "the generated records replace the hand
  structs, for EVERY table". The facade copies are a waypoint. The props (ItemProp 7,252 rows; MoverProp 985;
  skills 315) are read per hit, per AI tick and per frame (603 `GetItemProp(` sites, ~2,800 field reads [M per
  L-0147]). **The generated record layout becomes hot-path memory and cache behaviour.** It should be settled
  before L-0147 wave E.
- L-0147 also notes a lifetime contract: raw prop pointers are kept across frames, so the storage must never
  move for the process lifetime. Whatever container the generated table uses must guarantee stable addresses
  (or Sovereign must know it does not).

### Finding 5 -- duplication [M]

- **Runtime copies:**
  - `canon_runtime.h` is written 27 times (one md5), `canon_runtime_json.h` 25 times (one md5): 1.9 MB.
  - Go `rt/rt.go` is copied into 135 generated packages (~21.8 KB each, ~2.9 MB). Each is its own Go package, so
    binaries link duplicates [I].
- **Name tables:** in 332 of 340 enums the `ToName` table is identical to `ToWire` (35,986 duplicate entries).
  `FromWire` adds a third, sorted copy of the same strings plus a values array. Linker string pooling may merge
  the characters [I]; the pointer arrays and compile work stay.
- **One id set, five-plus representations.** The text ids exist as:
  1. the `TextId` enum;
  2. `TID_*` macros;
  3. `kNames`/`kWires`/`kKeys` tables;
  4. `header/ids/ids.gen.cpp` (33 k names/values), which `IdsAccess::Build()` copies into
     `std::vector<std::string>`, which Sovereign's `GeneratedDefines.cpp` copies again into
     `std::map<std::string,int>`;
  5. `Resource/Server/Ids/*.json` (1.1 MB, an `emit text` output -- no C++ reader found [M]);
  6. plus the Go and TS copies.
- **Baked data materialized twice:** 11 `Build()` functions copy constexpr tables into heap
  `std::vector<std::string>` on first use.
- **Services:**
  - Go: 277 generated files, 22.8 MB, of which 152 are byte-identical copies (`textvocab.gen.go` 2.73 MB exists
    in both admin and wiki).
  - TS: 9 duplicate pairs, and copies have drifted: `services.common.auth.ts` in admin/monitoring (Oct 29-30)
    differs from trello/website (Oct 14), i.e. services built from different Resource states.
  - Not a generator bug, but a shared generated module would end it.

### Finding 6 -- lookups [M]

- `KeyedList` = sorted `vector<std::string>` + index, string binary search.
- Enum ordinal lookup is a binary search even for the **100 dense enums whose codes are 0..n-1**.
- `TextId` is sparse (12,316 members over 17,848 codes); a dense code->ordinal table would be ~35 KB and O(1).
- No hash map and no id-indexed array in generated code. For hot-path tables keyed by a baked id enum (items by
  `ItemId`), an array indexed by ordinal is the obvious shape.

### Finding 7 -- header weight and build cost [M]

- A types header includes whole vocab headers for a few enums (`items.gen.h` pulls in the ai, character, engine,
  ui/neuz and ui vocabs). Vocab headers carry inline code and ordinal arrays (24 k lines for `TextId`) plus the
  guards.
- Generated TUs' generated-header closures total **256 MB** vs 16 MB of their own .cpp (16x). Weighted by exe:
  ~**596 MB of preprocessed generated text per full build**.
- 159 of the 276 TU instances pull `nlohmann/json.hpp` (917 KB).
- 241 hand-written files include non-shim generated headers (89 of them are headers, so it fans out further).
  Examples: `Project.cpp` 8.3 MB preprocessed, `RiftConfig.cpp` / `TowerConfig.cpp` / `DungeonResource.cpp`
  7.4 MB each.
- Generated .cpp sit in non-PCH lists. No unity build; each exe compiles its own copy. **104 unique generated
  TUs become 276 compile instances** (70 MB compiled vs 15.7 MB unique); one TU is compiled into 10 exes.
  (Partly a Sovereign CMake choice -- a shared static/OBJECT library is Source's to do -- but per-consumer
  pruning would be Canon's.)
- Neither L-0138 nor L-0147 records a size, compile-time or boot-time budget. Sovereign owes those.

### Finding 8 -- function size (the 0.1.11 stack incident) [M]

- `IdsAccess::Build` once held 33.5 k `std::string` temporaries in one frame, over the 1 MB Windows stack.
  0.1.11 fixed it with namespace-scope constexpr tables walked by a loop.
- Longest code bodies today:
  - `Decode(TroupeSkill)` 770 lines, `Decode(Item)` 624, `Decode(TargetsSolver)` 441;
  - `RecordbookAccess::Build` 334, still per-row `std::vector<std::string>{...}` blocks.
- Decode/Build size grows with record width, so the frame risk returns at 5-10x [I]. A per-function frame budget
  in the generator's own tests would catch it.

### Finding 9 -- TS output for a browser bundle [M]

- ~3.7 MB of generated TS across services (admin 0.83, monitoring 2.4, website 0.34, wiki 0.10, trello 0.06 MB).
- Named exports (good), but every table is wrapped in `Object.freeze` / `canonFreeze` / `new CanonMap`: 465 such
  calls in `services.admin.ts`.
  - None is annotated `/*#__PURE__*/`.
  - No services `package.json` sets `sideEffects`.
  - So bundlers likely keep unused tables [I].
- Each enum emits 5 tables (Members, LiveMembers, Names, Index, Codes); Names is an identity map.
- The browser client would import far more of Canon than any service does. Tree-shaking and per-entry lazy data
  are the difference between a fast first load and a multi-MB JS parse.

### Finding 10 -- finite `export fn` tables [M]

- `verdictOf` (14 Bool parameters) became a **16,384-cell `VERDICT_OF` literal, 292 KB, 47% of
  `services.admin.ts`**. `admin.gen.go` (1.34 MB) probably holds the same table [I].
- The `E9002` cap (65,536 cells) allows ~1.2 MB per function.
- A threshold above which the body is translated instead of tabulated (when the body is in the translatable
  subset) would turn 292 KB into ~1 KB.

### Finding 11 -- Canon itself at scale [M]

- 142 packages, 8,581 `.canon` files (7,252 are item entry files).
- Whole-project `canon check`: **31 s, ~2.0 GB RSS**, 0 findings.
- Budget 200,000,000 steps per package; `items` uses ~46 M.
- Resource README: `canon test` peaks ~7.7 GB under a 12 GB cap.
- Linear extrapolation at 5-10x: 2.5-5 min per whole check and over the memory cap [I].
- Per-package budgets scale by splitting. Whole-project time and memory do not.
- Previously recorded as owed: per-entry deps, on-disk cache (2026-10-07 and 2026-10-09 replies).

---

## 7. Tensions with Canon's laws -- for the Canon side to judge

### 7.1 Purity and asset hashes

The purity law allows only files under declared roots, read by `load`. Hashing an asset under a root is
deterministic, so it may be compatible in principle [I]. But it means reading gigabytes per check (models
~15 k files, textures ~15 k, world chunks).

Source's default assumption: **the baker owns hashes and the CDN manifest.** Canon references assets by logical
id with existence checks. A deploy step joins id -> hash. If Canon sees a better split (e.g. hashing only its own
outputs, which it already tracks in `canon.outputs`), say so.

### 7.2 Build-time only vs generated codecs

A wire codec is generated code that runs at runtime, like today's decoders. Source reads that as consistent with
"types, values, lookups and getters, plus translated `export fn`" extended by encoders. Whether encoders and
builders are a new law-level category is Canon's call.

### 7.3 The translated subset is narrow on purpose

The portable subset guarantees identical results per target. Source's asks (float32, pow, records, table
lookups, loops, bit ops) all widen it.

- Where a widening cannot be made bit-identical across C++/TS/Go (e.g. `pow` on float32 in JS has no `powf`),
  Source would rather have:
  - a curated, specified math set with documented per-target behaviour and conformance;
  - or a refusal with the reason.
  Not silent drift.
- Option for Canon to weigh: a float32 mode where `pow` is defined by a Canon-specified algorithm (not the
  platform `powf`), so all targets agree by construction.

### 7.4 One emit per target per package (`E8002`)

The browser and the server want different shapes of the same data:

- the browser wants per-entry lazy files and a pruned field set;
- the server wants a mapped blob of everything.

Source does not know whether the answer is several emits, consumer profiles, or package splits.

### 7.5 Release-permuted packet ids

Sovereign permutes packet ids per release build (anti-bot, L-0140). Only 15 ids are fixed. A stable-id registry
in Canon would carry the logical id. The permutation could stay a Source build step (a seeded remap applied
after generation), or Canon could emit a permutation table from a seed passed as a root input.

Note: the purity law forbids randomness, so the seed would be data. Source's default is to keep the permutation
on its side. The browser client weakens this measure anyway: the code is readable.

### 7.6 Flag-guarded protocol rows

220 of the id rows are guarded by `#ifdef` feature flags. `engine.features` already reads Sovereign's
`Config/Sys_*.h` through `load.text` (`features.canon:54`). Mapping flagged messages through it is untested.

Should a message whose flag is OFF be absent from both codecs (C++ and TS)? Source would say yes. The TS side
then needs the same flag set as the C++ build.

### 7.7 Earlier roadmap items

The 2026-10-12 roadmap to Louis (`2026-10-12-canon-next-versions.md` §3-§4, still open) lists v0.2 items
including "wire forms" and "M6 legacy C++". Some of the above may already be planned or refused there. Please
map each ask to those entries.

---

## 8. Generator optimizations, ranked (Source's suggestion, Canon's call)

| # | Change | Payoff | Notes |
|---|---|---|---|
| 1 | Drop the `*.defines.gen.h` shims and the push/pop guards; consumers use the enum or namespaced `constexpr` | -7.8 MB (27%) C++; 31 k global macros out of the PCHs | Sovereign does the call-site sweep. Canon provides the non-macro spelling and a switch to stop emitting shims per package |
| 2 | Split headers: per-package forward-declaration header (`enum class X : uint16_t;`); code/member arrays and the json include move to the .cpp | hand-written TU closures from 7-8 MB to <0.5 MB [I]; build preprocessing down ~10x [I] | biggest build-time win |
| 3 | Binary columnar data target: string table, fixed-width columns, presence bitsets, fingerprinted; C++ reads it memory-mapped through zero-copy views; TS reads the same bytes from an `ArrayBuffer` | `propItem` 25 MB -> ~2-3 MB [I]; boot decode seconds -> milliseconds [I]; **the browser's data format for free** | cheap interim: compact JSON (1.4-2.5x smaller, measured) and no `"="` none marker (absent key instead) |
| 4 | Record layout: presence bits instead of per-field `std::optional`; widths from refined ranges; `float` where declared Float32; stable addresses guaranteed | item row ~1.2 KB -> ~0.4 KB [I]; better cache behaviour on hot paths | **before L-0147 wave E.** Overlaps the checked-widths ask |
| 5 | One shared runtime per language (one C++ header pair, one Go module, one TS module) | -1.9 MB C++, -2.9 MB Go, no version skew | |
| 6 | Dense lookups: direct index for dense enums, code->ordinal table for sparse ones, perfect hash for `FromWire`, tables keyed by a baked id enum emitted as arrays | O(1) on hot paths | |
| 7 | Name-table dedupe: emit `ToWire` only when it differs from `ToName`; `FromWire` sorts a permutation index instead of a second string copy | ~-30% vocab .cpp [I] | |
| 8 | No materialized copies of baked data: expose `std::span` / `string_view`; a constexpr sorted table instead of Source building a `std::map` | removes boot heap copies (33 k strings for ids) | |
| 9 | Per-consumer pruning: emit only what a given exe/bundle imports (Source would declare consumers) | fewer TUs and smaller bundles; the 276 -> 104 compile cut needs Source's shared library too | |
| 10 | TS hygiene: `/*#__PURE__*/` on table construction, a `sideEffects` hint, fewer tables per enum, `bigint`/hi-lo for wide ranges, `Math.trunc` division in the safe range; table-vs-translate threshold for finite fns | smaller browser bundles, correct int64; `verdictOf` 292 KB -> ~1 KB | |
| 11 | Function-size budget in the generator's own tests (Decode/Build) | keeps the 0.1.11 stack class from returning | |
| 12 | Incremental/cached check per package; memory ceiling | keeps `canon check` usable at 5-10x | already owed per earlier replies |

---

## 9. What Sovereign would do on its side (so Canon knows the consumer's plan)

Not commitments; the likely shape if Louis schedules the direction.

1. **Generator hygiene first,** while output is 740 k lines: items 1, 2, 4, 5 above. Source sweeps the legacy
   macro call sites and moves generated code into one compiled library per language.
2. **Binary data target** next. The server boot benefits immediately, and it is the browser's data format.
3. **Wider translated subset.** Unblocks L-0148 phase 2 (tooltip instance formulas: upgrade, rarity, Azure,
   level_down) and removes three Go ports.
4. **Wire target + versioning + builders.** Migrated message by message, gated on `__NEUZ_MSGVR`, starting with
   the `CItemElem` and mover state blocks. Acceptance vehicle: a TS headless client (twin of `Apps/BotClient`)
   logging in over WebSocket and walking, on generated codecs only.
5. **Chunk splitting and manifest** alongside the new world format design (Sovereign side: a world source
   format, a baker, an editor that writes it).
6. In parallel on Source, no Canon impact:
   - WebSocket on Login/Cache, session resume on Cache, the trust audit;
   - the C++/WebAssembly numeric kernel;
   - the in-house SFX runtime and the Effekseer -> data converter;
   - the world baker.

---

## 10. Questions for Canon

1. Is a binary wire codec a Canon emit target, or a separate generator that consumes Canon's type model (e.g.
   via `canon graph` / a typed export)? What does Canon gain or lose either way?
2. Can one binary data format serve C++ (mmap, zero-copy), TS (`ArrayBuffer`) and Go, with one fingerprint? Is
   `data` mode the natural home, given no package uses it yet?
3. Float32: a new arithmetic type, a package-level numeric mode, or a refusal? How would `pow` be specified so
   C++, TS and Go agree?
4. Protocol versioning: how does it fit `stable` / `past` / `retired` and `canon.lock`? Can a record carry
   "present since version N" with the version a Canon const?
5. Audience projections of one state record (owner / others / DB / inter-server): first-class, or several
   records sharing a base?
6. Runtime builders/encoders for `types` records: acceptable under the build-time-only law as "generated
   codecs"?
7. Per-consumer pruning and per-entry outputs vs `E8002`: profiles, multiple emits, or package splits?
8. Scale: is a 5-10x project (protocol, world metadata, ~2,800 effect definitions) inside what Canon intends to
   support without the on-disk cache and per-entry deps? Which of those comes first?
9. Which of §8's optimizations are cheap in the current generator, and which need a generator redesign?

---

## Paths

- **Source:**
  - `Engine/Resource/Generated/**`, `Engine/Telemetry/Generated/**`, `Engine/Service/Notif/Generated/**` (output);
  - `Engine/Net/Packets/packet_ids.tsv` + `Tools/build/gen_packet_ids.py` (id registry);
  - `Engine/Net/Misc/Archive/ar.{h,cpp}` (CAr);
  - `Engine/Net/Transport/`, `Engine/Net/Cipher/`;
  - `Engine/Serialization/ObjSerialize.cpp`, `ObjSerializeOpt.cpp`, `NMSerialization.cpp`;
  - `Client/Neuz/DPClient.cpp`, `Server/World/Network/DPSrvr.cpp`;
  - `Apps/BotClient/`.
- **Source ADRs:**
  - `meta/decisions/L-0138-canon-generated-cpp-structures-and-loaders.md`;
  - `L-0147-prop-structures-over-canon-records.md` (rulings 2026-10-30);
  - `L-0148-item-tooltip-content-from-canon.md` (DRAFT);
  - `L-0120-world-terrain-campaign.md`;
  - `L-0125-penya-int64.md`;
  - `L-0140-anti-bot-hardening.md`.
- **Resource:**
  - `Config/**` (142 packages);
  - `Config/items/blob/` (the hand-rolled varint/bit workaround);
  - `Config/text/client/emit.canon` (no TS emit);
  - `Server/*.json` (43.3 MB rendered data).
- **Services:** `*/web/src/lib/generated/*.ts`, generated Go under each service (duplicated copies, finding 5).
- **Related handoffs:**
  - `2026-10-29-sovereign-cpp-macro-collisions.md`;
  - `2026-10-30-sovereign-cpp-checked-widths.md`;
  - `2026-10-12-canon-next-versions.md` §3-§4 (open roadmap).
