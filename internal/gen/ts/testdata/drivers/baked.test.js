import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as a from "./baked/features/a/a.js";

test("tables and keyed lists find, at and iterate in source order (CODEGEN.md §5.9)", () => {
  assert.equal(a.skills.length, 2);
  assert.equal(a.skills.at(0).id, "fireball");
  assert.equal(a.skills.find("aegis").side, "RIGHT");
  assert.equal(a.skills.find("nobody"), null);
  assert.deepEqual(a.statuses.all.map((s) => s.id), ["open", "closed", "stale"]);
  assert.equal(a.statuses.find("stale").retired, true);
  assert.equal(a.statuses.find("open").retired, false);
  assert.deepEqual(a.StatusIdIndex, { open: 0, closed: 1, stale: 2 });
});

test("values are deep-frozen, maps refuse writes (CODEGEN.md §8.1, TS-01)", () => {
  const f = a.skills.find("fireball");
  assert.ok(Object.isFrozen(a.skills));
  assert.ok(Object.isFrozen(f));
  assert.ok(Object.isFrozen(f.flags));
  assert.ok(Object.isFrozen(f.kind));
  assert.ok(Object.isFrozen(a.objective.target));
  assert.throws(() => f.weights.set("FIRE", 1), TypeError);
  assert.throws(() => f.counts.delete(1), TypeError);
  assert.throws(() => {
    "use strict";
    f.reqMp = 1;
  }, TypeError);
  assert.ok(Object.isFrozen(a.LEVELS));
  assert.throws(() => a.SCALES.set("c", 1), TypeError);
});

test("literals: precomputed fns are properties, a finite fn a frozen table (CODEGEN.md §5.10)", () => {
  const f = a.skills.find("fireball");
  assert.equal(f.isFree, false);
  assert.equal(a.skills.find("aegis").isFree, true);
  assert.equal(f.resist.WATER, 2);
  assert.ok(Object.isFrozen(f.resist));
  assert.deepEqual({ ...f.kind }, { kind: "weapon", attack: 5, affinity: null });
  assert.equal(f.castTime, 2000);
  assert.deepEqual({ ...a.objective.target }, { branch: "collect", value: 3 });
});

test("constants of every kind (CODEGEN.md §5.1)", () => {
  assert.equal(a.LIMIT, 50);
  assert.equal(a.RATE, 0.25);
  assert.equal(a.NAME, "canon");
  assert.equal(a.TICK, 90000);
  assert.equal(a.SIDE, "RIGHT");
  assert.deepEqual([...a.LEVELS], [1, 2, 3]);
  assert.equal(a.SCALES.get("b"), 2.5);
});

test("package-level fns: a precomputed value, lookups by ordinal (CODEGEN.md §5.10)", () => {
  assert.deepEqual([...a.defaultFlags()], ["tradable", "droppable"]);
  assert.equal(a.storable("tradable"), true);
  assert.equal(a.storable("soulbound"), false);
  assert.equal(a.nextOf("open", true), "closed");
  assert.equal(a.nextOf("open", false), null);
  assert.equal(a.nextOf("stale", true), "closed");
});

test("enum tables (CODEGEN.md §5.2)", () => {
  assert.deepEqual([...a.SideMembers], ["left", "RIGHT"]);
  assert.equal(a.SideNames.RIGHT, "right");
  assert.equal(a.SideIndex.RIGHT, 1);
  assert.deepEqual({ ...a.ElementCodes }, { FIRE: 1, WATER: 2, WIND: 3 });
  assert.ok(Object.isFrozen(a.SideMembers));
});
