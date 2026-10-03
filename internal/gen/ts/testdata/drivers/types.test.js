import { test } from "node:test";
import * as assert from "node:assert/strict";
import { decodeObjective, decodeSkill, decodeKind, decodeStatBonus, parseObjective, parseSkill, parseKind, parseStatBonus } from "./types/features/a/a.js";

test("a types-mode decoder reads the source wire (CODEGEN.md §5.13, WIRE.md §5.13)", () => {
  const s = decodeSkill({
    dwID: "fireball",
    legacy: { reqMp: 3 },
    element: 1,
    flags: 5,
    bTwoHanded: 1,
    castTime: 1.5,
    cast_time: 1.5,
    weights: { 1: 0.5, 2: 1.25 },
    counts: { 3: 4 },
    dwDestParam0: "STR",
    nAdjParamVal0: 5,
    dwDestParam1: "DEX",
    nAdjParamVal1: 6,
    type: "weapon",
    attack: 9,
    unknown: "ignored",
  });
  assert.equal(s.id, "fireball");
  assert.equal(s.reqMp, 3);
  assert.equal(s.reqFp, 0);
  assert.equal(s.element, "FIRE");
  assert.deepEqual([...s.flags], ["tradable", "soulbound"]);
  assert.equal(s.twoHanded, true);
  assert.equal(s.side, "both");
  assert.equal(s.castTime, 1500);
  assert.equal(s.weights.get("WATER"), 1.25);
  assert.equal(s.counts.get(3), 4);
  assert.deepEqual(s.stats.map((b) => ({ ...b })), [{ attribute: "STR", value: 5 }, { attribute: "DEX", value: 6 }]);
  assert.deepEqual({ ...s.kind }, { kind: "weapon", attack: 9, affinity: null });
  assert.deepEqual([...s.goals], []);
  assert.ok(Object.isFrozen(s) && Object.isFrozen(s.stats));
});

test("none is absent, null or the marker; a default fills an absent key (WIRE.md §5.4)", () => {
  const base = { dwID: "a", type: "plain" };
  assert.equal(decodeSkill(base).element, null);
  assert.equal(decodeSkill({ ...base, element: null }).element, null);
  assert.equal(decodeSkill({ ...base, element: 0 }).element, null);
  assert.equal(decodeSkill({ ...base, castTime: null }).castTime, null);
  assert.equal(decodeSkill(base).kind.kind, "plain");
  assert.equal(decodeSkill({ ...base, element: 2 }).element, "WATER");
});

test("variants and dependent values (WIRE.md §5.6, §5.9)", () => {
  assert.deepEqual({ ...decodeKind({ type: "armor", defense: 2 }) }, { kind: "armor", defense: 2 });
  assert.deepEqual({ ...decodeKind({ type: "junk" }) }, { kind: "junk" });
  assert.throws(() => decodeKind({ type: "nope" }), /^Error: \/type: unknown case/);
  assert.throws(() => decodeKind({ attack: 1 }), /\/type: expected a string/);
  assert.deepEqual({ ...decodeObjective({ goal: "kill", target: "wolf" }).target }, { branch: "kill", value: "wolf" });
  assert.equal(decodeObjective({ goal: "visit" }).target, null);
  assert.throws(() => decodeObjective({ goal: "visit", target: 1 }), /\/target: no branch for this value/);
  assert.throws(() => decodeObjective({ goal: "kill", target: 1 }), /\/target: expected a string/);
  assert.throws(() => decodeObjective({ goal: "nope" }), /\/goal: unknown member/);
});

test("anything it cannot represent fails, naming the path (CODEGEN.md §5.13)", () => {
  assert.throws(() => decodeObjective(null), /^Error: \/: expected an object/);
  assert.throws(() => decodeObjective([]), /expected an object/);
  assert.throws(() => decodeStatBonus({ attribute: "a", value: 1.5 }), /\/value: expected an integer/);
  assert.throws(() => decodeStatBonus({ attribute: "a", value: 9007199254740993 }), /\/value: integer outside the TypeScript safe range/);
  assert.throws(() => decodeStatBonus({ attribute: 1, value: 1 }), /\/attribute: expected a string/);
  assert.equal(decodeStatBonus({ attribute: "a", value: -0 }).value, 0);
  assert.ok(Object.is(decodeStatBonus({ attribute: "a", value: -0 }).value, 0));
});

test("types mode has parse<T> beside decode<T>, with parse's exactness (CODEGEN.md §8.1, DECISIONS 278)", () => {
  assert.deepEqual({ ...parseStatBonus('{"attribute": "a", "value": 2}') }, { attribute: "a", value: 2 });
  assert.throws(() => parseStatBonus('{"attribute": "a", "value": 2.0}'), /\/value: expected an integer/);
  assert.equal(decodeStatBonus({ attribute: "a", value: 2.0 }).value, 2);
  assert.throws(() => parseStatBonus('{"attribute": "a", "value": 9007199254740993}'), /integer outside the TypeScript safe range/);
  const s = parseSkill('{"dwID": "a", "type": "plain", "counts": {"3": 4, "1": 2}, "cast_time": 1.50000000000000000000000, "element": 0.0}');
  assert.deepEqual([...s.counts.keys()], [3, 1]);
  assert.equal(s.castTime, 1500);
  assert.equal(s.element, null);
  assert.deepEqual({ ...parseKind('{"type": "armor", "defense": 2}') }, { kind: "armor", defense: 2 });
  assert.deepEqual({ ...parseObjective('{"goal": "kill", "target": "wolf"}').target }, { branch: "kill", value: "wolf" });
  assert.throws(() => parseObjective('{"goal": "kill",}'), Error);
  assert.ok(Object.isFrozen(s));
});
