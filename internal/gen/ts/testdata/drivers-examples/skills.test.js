import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { decodeSkills, parseSkills, roundHalf, addInt, divInt } from "./features/ts/expected/ts/out/skills.generated.js";

const text = readFileSync(new URL("./features/ts/expected/ts/out/skills.json", import.meta.url), "utf8");

test("features.ts: the data file decodes, a bigint field included", () => {
  const skills = parseSkills(text);
  assert.equal(skills.length, 3);
  assert.equal(skills.find("cleave").serial, 9007199254740993n);
  assert.equal(skills.find("slash").serial, 0n);
  assert.equal(skills.find("slash").next, "cleave");
  assert.equal(skills.find("slash").weights.get("fire"), 2);
  assert.deepEqual({ ...skills.find("cleave").reward }, { kind: "gold", amount: 500 });
  assert.deepEqual({ ...skills.find("storm").reward }, { kind: "item", define: "II_STORM", count: 3 });
  assert.equal(skills.find("cleave").note, "area");
  assert.equal(skills.find("storm").rarity, "epic");
});

test("features.ts: decode sees a JSON.parse value, which cannot hold the bigint (CODEGEN.md §8.1)", () => {
  assert.throws(() => decodeSkills(JSON.parse(text)), /\/rows\/1\/serial: integer outside the TypeScript safe range/);
});

test("features.ts: translated fns signal the evaluator's codes (CONFORMANCE.md §3, §4)", () => {
  const code = (f) => {
    try {
      f();
    } catch (e) {
      return e.code;
    }
    return "";
  };
  assert.equal(addInt(9007199254740990, 1), 9007199254740991);
  assert.equal(code(() => addInt(9007199254740991, 1)), "E8303");
  assert.equal(code(() => divInt(1, 0)), "E4102");
  assert.equal(divInt(-7, 2), -3);
  assert.equal(roundHalf(-2.5), -3);
  assert.equal(code(() => roundHalf(1e300)), "E4103");
});
