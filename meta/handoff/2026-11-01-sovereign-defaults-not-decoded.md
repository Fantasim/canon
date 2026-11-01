# Sovereign -> Canon: some field defaults are not applied by the generated C++ decoder (2026-11-01)

**From:** Sovereign Source (L-0147 wiki handoffs, units 4c / 10a / 7). **To:** Canon. **Status:** OPEN.

## What we hit (canon 0.1.13, `cpp` emit)

`canon guide wire`: "An absent key is the field's default". The generated C++ decoder does not apply that rule in
these three cases:

1. **A nested record whose default holds an enum value.** `bossKill: BossKillReward = {}` where `BossKillReward` has
   `item: LiveItem = II_CURRENCY_RAID`: the decoder emits `dec.Required(v, "bossKill")` and no default, so a file
   without the key is refused. With only scalar or vector fields in the nested record, the default is applied.
   (Sovereign worked around it with flat fields on the parent: Resource `Config/content/propDungeon.canon`
   `DungeonGlobal.bossCreditMinDamage / bossCurrencyItem / bossCurrencyTiers`.)
2. **A non-empty list default** (`wardcoinScrolls: [WardcoinScroll] = [ ... ]`): the key becomes Required; the field
   had to be optional with no default (Resource `Config/items/catalog/wardrobe.canon`).
3. **Bool default `true`:** `playable: Bool = true` decodes to true when the key is absent (correct), but the generated
   class member is `bool playable_ = false;` (`ai/npc.gen.h`), so a default-constructed record disagrees with the
   decoder and with the Canon default.

## Why it matters

Sovereign's release rule: an old exe must load the new file AND a new exe must load the old file. A type default equal
to today's value is how a new field ships safely; when the decoder makes it Required, a new exe refuses an older
rendered file (boot abort for boot-loaded files).

## Ask

Apply every declared default in the generated decoders (C++, Go, TS) for absent keys, including nested records with
enum fields and non-empty list defaults, and initialise generated members to the Canon default. A conformance test per
case (absent key -> default) in the emitters' suites.

Also noted (lower priority): enum-keyed lookups emitted from a `local` table call `std::abort()` on an unknown code
(`combat/vocab.gen.h` Sovereign display lookups). Sovereign guards every caller; a non-aborting form (or a documented
NUL-terminated `string_view` contract for string cells) would make the generated surface safer by default.
