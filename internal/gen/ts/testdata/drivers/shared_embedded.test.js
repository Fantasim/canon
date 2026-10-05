import { test } from "node:test";
import * as assert from "node:assert/strict";
import { zones, bands, moods, medals, start, perks, shelf } from "./shared_embedded/features/world/world.js";
import { badges, statuses } from "./shared_embedded/features/core/core.js";

test("a table of another package's record holds rows with this package's ids (CODEGEN.md §5.9, §8.1, DECISIONS 323)", () => {
  assert.deepEqual({ ...bands.find("low") }, { id: "low", retired: false, min: 1, max: 5, labels: [], tone: "soft", status: "open", width: 4 });
  assert.equal(bands.find("old").retired, true);
  assert.deepEqual(medals.all.map((m) => [m.id, m.retired, m.label]), [["bronze", false, "Bronze"], ["tin", true, "Tin"]]);
  assert.equal(medals.find("gold"), null);
  assert.equal(badges.find("gold").id, "gold");
  assert.deepEqual(moods.all.map((m) => [m.id, m.retired, m.label]), [["calm", false, "Calm"], ["grim", true, "Grim"]]);
  assert.equal(moods.find("open"), null);
  assert.equal(statuses.find("open").id, "open");
  assert.ok(Object.isFrozen(bands.find("low")));
});

test("a table field, a plain value, a variant and dependent values of another package's types (CODEGEN.md §2.8, §4.2)", () => {
  const z = zones.find("forest");
  assert.deepEqual(z.markers.map((m) => [m.id, m.retired, m.code]), [["gate", false, "g"], ["well", true, "w"]]);
  assert.deepEqual({ ...z.reward }, { kind: "coins", amount: 3 });
  assert.deepEqual({ ...z.effect.bonus }, { branch: "gold", value: 2 });
  assert.deepEqual({ ...start }, { min: 1, max: 2, labels: [], tone: "loud", status: "open", width: 1 });
  assert.equal("id" in start, false);
  assert.deepEqual({ ...perks.find("rich").bonus }, { branch: "gold", value: 7 });
});

test("a table inside another package's record keeps its rows' ids (CODEGEN.md §5.4, DECISIONS 279(c))", () => {
  assert.deepEqual(shelf.slots.map((m) => [m.id, m.retired, m.code]), [["top", false, "t"], ["low", true, "l"]]);
});
