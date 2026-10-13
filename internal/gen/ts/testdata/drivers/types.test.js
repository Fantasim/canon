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
  assert.throws(() => decodeKind({ type: "nope" }), /^Error: Kind: \$\.type: unknown case/);
  assert.throws(() => decodeKind({ attack: 1 }), /\$\.type: expected a string/);
  assert.deepEqual({ ...decodeObjective({ goal: "kill", target: "wolf" }).target }, { branch: "kill", value: "wolf" });
  assert.equal(decodeObjective({ goal: "visit" }).target, null);
  assert.throws(() => decodeObjective({ goal: "visit", target: 1 }), /\$\.target: no branch for this value/);
  assert.throws(() => decodeObjective({ goal: "kill", target: 1 }), /\$\.target: expected a string/);
  assert.throws(() => decodeObjective({ goal: "nope" }), /\$\.goal: unknown member/);
});

test("anything it cannot represent fails, naming the path (CODEGEN.md §5.13)", () => {
  assert.throws(() => decodeObjective(null), /^Error: Objective: \$: expected an object/);
  assert.throws(() => decodeObjective([]), /expected an object/);
  assert.throws(() => decodeStatBonus({ attribute: "a", value: 1.5 }), /value: expected an integer/);
  assert.throws(() => decodeStatBonus({ attribute: "a", value: 9007199254740993 }), /value: integer outside the TypeScript safe range/);
  assert.throws(() => decodeStatBonus({ attribute: 1, value: 1 }), /\$\.attribute: expected a string/);
  assert.equal(decodeStatBonus({ attribute: "a", value: -0 }).value, 0);
  assert.ok(Object.is(decodeStatBonus({ attribute: "a", value: -0 }).value, 0));
});

test("types mode has parse<T> beside decode<T>, with parse's exactness (CODEGEN.md §8.1, DECISIONS 278)", () => {
  assert.deepEqual({ ...parseStatBonus('{"attribute": "a", "value": 2}') }, { attribute: "a", value: 2 });
  assert.throws(() => parseStatBonus('{"attribute": "a", "value": 2.0}'), /value: expected an integer/);
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

test("parse<T> names a syntax error's UTF-8 byte offset, the same vectors as every target (CODEGEN.md §5.13, DECISIONS 361)", () => {
  const vectors = [
    ['{"d": ', 6], ['{"d": x}', 6], ["{} {}", 3], ["{} 123", 3], ['{"a": tru}', 9], ['{"d":"hp"} x', 11],
    ['\ufeff{"d" 1}', 8], ["[,", 1], ["", 0], ["\ufeff", 3], ['{"a":"é" x', 10], ['{"a":"😀" x', 12], ["01", 1], ["1.", 2], ["-", 1],
    ['"\\q"', 2], ['"\\u12G4"', 5], ['"a\tb"', 2], ['{"a":1,}', 7], ["[\ud800]", 1], ['{"a": "\\ud800', 13], ['{"a": "\\ud800\\', 14],
  ];
  for (const [text, at] of vectors) {
    assert.throws(() => parseStatBonus(text), new Error("StatBonus: invalid JSON at byte " + at), JSON.stringify(text));
  }
});

test("parse<T> names a byte rule's failure at its $ path, the first failure in document order (CODEGEN.md §5.13, DECISIONS 361)", () => {
  const vectors = [
    ['{"a":1,"a":2,', "$.a: duplicate key"],
    ['{"x":{"y":1,"y":2}}', "$.x.y: duplicate key"],
    ['{"x":[{"y":1},{"z":1,"z":2}]}', "$.x[1].z: duplicate key"],
    ['{"a":1,"\\u0061":2}', "$.a: duplicate key"],
    ['{"a":"\\ud800"}', "$.a: unpaired surrogate"],
    ['{"a": "\\udc00', "$.a: unpaired surrogate"],
    ['{"a": "\\ud800 x', "$.a: unpaired surrogate"],
    ['"\\ud800A"', "$: unpaired surrogate"],
    ['{"a": "\\ud800\\u0041"}', "$.a: unpaired surrogate"],
    ['{"a": "\ud800"}', "$.a: not UTF-8"],
    ['{"\ud800":1}', "$: not UTF-8"],
    ['{"a":["\\udc00"]}', "$.a[0]: unpaired surrogate"],
    ["[".repeat(600), "$" + "[0]".repeat(512) + ": nested deeper than 512 levels"],
    ['{"a":1,"a":2} x', "$.a: duplicate key"],
  ];
  for (const [text, want] of vectors) {
    assert.throws(() => parseStatBonus(text), new Error("StatBonus: " + want), JSON.stringify(text));
  }
  const deep = (n) => "[".repeat(n) + "]".repeat(n);
  assert.throws(() => parseStatBonus(deep(512)), new Error("StatBonus: $: expected an object"));
  assert.throws(() => parseStatBonus(deep(513)), /nested deeper than 512 levels/);
});

test("parse<T> accepts an escaped surrogate pair, in a value and a key (WIRE.md §3.2)", () => {
  assert.equal(parseStatBonus('{"attribute": "\\ud83d\\ude00", "value": 1}').attribute, "\u{1f600}");
  assert.equal(parseStatBonus('{"attribute": "a", "value": 1, "\\ud83d\\ude00": 2}').value, 1);
});

test("a literal union over a closed enum refuses any other string, as the Go decoder does (CODEGEN.md §5.13, WIRE.md §5.9)", () => {
  const skill = (side) => ({ dwID: "a", type: "plain", ...(side === undefined ? {} : { side }) });
  assert.equal(decodeSkill(skill("left")).side, "left");
  assert.equal(decodeSkill(skill("RIGHT")).side, "RIGHT");
  assert.equal(decodeSkill(skill("both")).side, "both");
  assert.equal(decodeSkill(skill()).side, "both");
  assert.throws(() => decodeSkill(skill("up")), new Error("Skill: $.side: unknown value up"));
  assert.throws(() => decodeSkill(skill("right")), new Error("Skill: $.side: unknown value right"));
  assert.throws(() => decodeSkill(skill(3)), new Error("Skill: $.side: expected a string"));
});

test("decode<T> names its failures behind <T>: and a $ path (CODEGEN.md §5.13, DECISIONS 361)", () => {
  assert.throws(() => decodeStatBonus(null), new Error("StatBonus: $: expected an object"));
  assert.throws(() => decodeStatBonus({ attribute: "a", value: "x" }), new Error("StatBonus: $.value: expected an integer"));
  assert.throws(() => parseObjective('{"goal": "nope"}'), new Error("Objective: $.goal: unknown member"));
});
