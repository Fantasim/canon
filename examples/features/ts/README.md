# ts

The TypeScript feature example (IMPLEMENTATION-PLAN §7.9, M6). It checks clean; `expected/` holds
the compiler's findings and its `emit json` output.

- `emit ts` in `data` mode: `decode<skills>(json)` over the emitted `skills.json`, no I/O (CODEGEN §8.1)
- records (`Skill`), a variant (`Reward`), an enum (`Rarity`), an optional (`note`), a map (`weights`), a keyed list (`skills`) and a ref (`next`)
- `@ts(bigint)` on `Skill.serial`, whose value exceeds 2^53 (CODEGEN §4.1, `E8101` without it)
- `@ts(name:)` renames the type `Skill` to `SkillEntry` and the fn `roundOf` to `roundHalf` (CODEGEN §3.5)
- translated `export fn`s: `addInt`, `subInt`, `mulInt`, `divInt`, `modInt` on negatives and near 2^53 (CONFORMANCE §4)
- `clampInt`, `toFloat`, `toInt` and `roundOf`: clamp and the int/float conversions
