import { test } from "node:test";
import * as assert from "node:assert/strict";
import { decodeZone, parseZone, decodePerk } from "./shared_types/features/world/world.js";

const zone = () => ({
  levels: { min: 1, max: 5, tone: "soft", status: "open" },
  reward: { kind: "coins", amount: 3 },
  markers: { gate: { code: "g" }, well: { code: "w", $retired: true } },
  effect: { aim: "mood", bonus: "loud" },
});

test("a types-mode decoder reads another package's classes with this file's own readers (CODEGEN.md §2.8, §5.13, DECISIONS 323)", () => {
  const z = decodeZone(zone());
  assert.deepEqual({ ...z.levels }, { min: 1, max: 5, labels: [], tone: "soft", status: "open" });
  assert.deepEqual(z.markers.map((m) => [m.id, m.retired, m.code]), [["gate", false, "g"], ["well", true, "w"]]);
  assert.deepEqual({ ...z.reward }, { kind: "coins", amount: 3 });
  assert.deepEqual({ ...z.effect.bonus }, { branch: "mood", value: "loud" });
  assert.deepEqual({ ...parseZone(JSON.stringify(zone())).effect }, { aim: "mood", bonus: { branch: "mood", value: "loud" } });
  assert.deepEqual({ ...decodePerk({ aim: "gold", bonus: 7 }).bonus }, { branch: "gold", value: 7 });
});

test("a failure inside another package's class names its JSON path (CODEGEN.md §5.13)", () => {
  const code = zone();
  code.markers.gate.code = 3;
  assert.throws(() => decodeZone(code), /^Error: \/markers\/gate\/code: expected a string$/);
  const tone = zone();
  tone.levels.tone = "mute";
  assert.throws(() => decodeZone(tone), /^Error: \/levels\/tone: unknown member$/);
  const entry = zone();
  entry.levels.status = "nope";
  assert.throws(() => decodeZone(entry), /^Error: \/levels\/status: no entry nope$/);
  const shut = zone();
  shut.levels.status = "shut";
  assert.equal(decodeZone(shut).levels.status, "shut");
  const kind = zone();
  kind.reward = { kind: "gems" };
  assert.throws(() => decodeZone(kind), /^Error: \/reward\/kind: unknown case$/);
});
