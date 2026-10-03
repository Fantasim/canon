import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { decodeSkills, decodeStatuses, decodeObjective, decodeMarks, parseSkills, parseStatuses, parseObjective, parseMarks } from "./data/features/a/a.js";

const text = (name) => readFileSync(new URL("./data/json/" + name, import.meta.url), "utf8");
const load = (name) => JSON.parse(text(name));

test("skills decode: every wire form", () => {
  const skills = decodeSkills(load("skills.json"));
  assert.equal(skills.length, 2);
  const f = skills.find("fireball");
  assert.equal(f.reqMp, 12);
  assert.equal(f.reqFp, 0);
  assert.equal(f.element, "FIRE");
  assert.deepEqual([...f.flags], ["tradable", "soulbound"]);
  assert.equal(f.twoHanded, false);
  assert.equal(f.side, "both");
  assert.equal(f.castTime, 2000);
  assert.equal(f.kind.kind, "weapon");
  assert.equal(f.kind.attack, 5);
  assert.equal(f.kind.affinity, null);
  assert.equal(f.isFree, false);
  assert.deepEqual({ ...f.resist }, { FIRE: 1, WATER: 2, WIND: 3 });
  const a = skills.find("aegis");
  assert.equal(a.side, "RIGHT");
  assert.equal(a.element, null);
  assert.equal(a.castTime, null);
  assert.equal(a.isFree, true);
  assert.deepEqual({ ...a.kind }, { kind: "armor", defense: 3 });
  assert.deepEqual(a.goals.map((g) => ({ ...g })), [
    { goal: "kill", target: { branch: "kill", value: "wolf" } },
    { goal: "visit", target: null },
  ]);
  assert.equal(skills.at(1), a);
  assert.equal(skills.find("nobody"), null);
});

test("decoded values are read-only", () => {
  const f = decodeSkills(load("skills.json")).find("fireball");
  assert.ok(Object.isFrozen(f));
  assert.ok(Object.isFrozen(f.flags));
  assert.ok(Object.isFrozen(f.kind));
  assert.throws(() => f.weights.set("FIRE", 1), TypeError);
  assert.throws(() => f.counts.clear(), TypeError);
});

test("a table keeps retired rows and finds by id", () => {
  const statuses = decodeStatuses(load("statuses.json"));
  assert.equal(statuses.length, 3);
  assert.equal(statuses.at(0).id, "open");
  assert.deepEqual([...statuses.find("open").next], ["closed"]);
  assert.equal(statuses.find("stale").retired, true);
  assert.equal(statuses.find("open").retired, false);
});

test("a record value", () => {
  const o = decodeObjective(load("objective.json"));
  assert.equal(o.goal, "collect");
  assert.deepEqual({ ...o.target }, { branch: "collect", value: 3 });
});

test("failures name the JSON path (CODEGEN.md §5.13)", () => {
  const bad = () => load("skills.json");
  assert.throws(() => decodeSkills({ ...load("skills.json"), $schema: "a.Skill@00000000" }), /built from schema a.Skill@00000000/);
  assert.throws(() => decodeSkills(5), /built from schema <none>/);
  let doc = bad();
  doc.rows[0].legacy.reqMp = "x";
  assert.throws(() => decodeSkills(doc), /^Error: \/rows\/0\/legacy\/reqMp: expected an integer/);
  doc = bad();
  delete doc.rows[1].dwID;
  assert.throws(() => decodeSkills(doc), /\/rows\/1\/dwID: expected a string/);
  doc = bad();
  doc.rows[0].type = "nope";
  assert.throws(() => decodeSkills(doc), /\/rows\/0\/type: unknown case/);
  doc = bad();
  delete doc.rows[0].dwDestParam0;
  doc.rows[0].dwDestParam1 = "x";
  doc.rows[0].nAdjParamVal1 = 1;
  assert.throws(() => decodeSkills(doc), /filled slot after an empty one/);
  doc = bad();
  doc.rows[0].element = 9;
  assert.throws(() => decodeSkills(doc), /\/rows\/0\/element: unknown code/);
  doc = bad();
  doc.rows[0].bTwoHanded = 2;
  assert.throws(() => decodeSkills(doc), /\/rows\/0\/bTwoHanded: expected 0 or 1/);
  doc = bad();
  doc.rows[0].flags = 8;
  assert.throws(() => decodeSkills(doc), /\/rows\/0\/flags: unknown bits/);
  doc = bad();
  doc.rows[0].cast_time = 0.0004;
  assert.throws(() => decodeSkills(doc), /not a whole number of milliseconds/);
});

test("parse reads the file text exactly (CODEGEN.md §8.1, DECISIONS 278)", () => {
  const skills = parseSkills(text("skills.json"));
  assert.equal(skills.find("fireball").castTime, 2000);
  assert.deepEqual([...skills.find("fireball").counts.keys()], [3, 1, 2]);
  assert.deepEqual([...parseStatuses(text("statuses.json")).all.map((s) => s.id)], ["open", "closed", "stale"]);
  assert.equal(parseObjective(text("objective.json")).goal, "collect");
});

test("decode sees a parsed value: integer-like keys come in JavaScript's order", () => {
  const skills = decodeSkills(load("skills.json"));
  assert.deepEqual([...skills.find("fireball").counts.keys()], [1, 2, 3]);
});

test("parse refuses a number token of the wrong form for an Int (E7103) and keeps durations exact", () => {
  const reqMp = (token) => parseSkills(text("skills.json").replace('"reqMp": 12', '"reqMp": ' + token));
  assert.equal(reqMp("13").find("fireball").reqMp, 13);
  for (const bad of ["2.0", "1e3", "-1.5"]) assert.throws(() => reqMp(bad), /expected an integer/, bad);
  const dur = (token) => parseSkills(text("skills.json").replace(/"cast_time": [0-9.e+-]+/, '"cast_time": ' + token)).find("fireball").castTime;
  assert.equal(dur("1.1"), 1100);
  assert.equal(dur("1.5e0"), 1500);
  assert.equal(dur("2.0"), 2000);
  assert.equal(dur("0.001"), 1);
  assert.throws(() => dur("0.0005"), /not a whole number of milliseconds/);
  assert.throws(() => dur("1e-4"), /not a whole number of milliseconds/);
  assert.throws(() => dur("9223372036855"), /duration out of range/);
  assert.equal(dur("1" + "0".repeat(70) + "e-70"), 1000);
  assert.equal(dur("0." + "0".repeat(80) + "1e81"), 1000);
  assert.throws(() => dur("1e-999999999"), /not a whole number of milliseconds/);
  assert.throws(() => dur("1e999999999"), /duration out of range/);
});

test("parse is strict JSON (WIRE.md §3)", () => {
  const bad = ["", "{", "[1,]", '{"a":1,}', "01", ".5", "+1", "NaN", "{'a':1}", '{"a":1 "b":2}', "[1] 2", '"\\u00"', '"\\ud800"', '"\\x"', '"a\tb"'];
  for (const t of bad) assert.throws(() => parseObjective(t), Error, t);
  assert.throws(() => parseObjective('{"$schema": "a.Objective@6eaf8bca", "$schema": "x"}'), /duplicate key \$schema/);
  assert.throws(() => parseObjective("[".repeat(600)), /nesting deeper than 512/);
  assert.equal(parseObjective("﻿" + text("objective.json")).goal, "collect");
});

test("none markers are equal by exact value, whatever the token (WIRE.md §5.4)", () => {
  const marks = parseMarks(text("marks.json"));
  const none = marks.find("none");
  assert.deepEqual([none.half, none.minus, none.big], [null, null, null]);
  const set = marks.find("set");
  assert.deepEqual([set.half, set.minus, set.big], [1.5, 2, 9007199254740993n]);
  const row = (tokens) => parseMarks(text("marks.json").replace(/\{"key": "none"[^}]*\}/, '{"key": "none", ' + tokens + "}")).find("none");
  const r = row('"half": 5e-1, "minus": -1, "big": 9.223372036854775807e18');
  assert.deepEqual([r.half, r.minus, r.big], [null, null, null]);
  const near = row('"half": 0.5000000000000001, "minus": -1.0000000000000001, "big": 9223372036854775806');
  assert.deepEqual([near.half, near.minus, near.big], [0.5000000000000001, -1, 9223372036854775806n]);
  assert.throws(() => row('"big": 9223372036854775808'), /integer out of range/);
});

test("decode sees numbers by value: a float marker matches, a bigint past 2^53 is refused", () => {
  const doc = load("marks.json");
  assert.equal(doc.rows[0].half, 0.5);
  const none = decodeMarks({ ...doc, rows: [{ ...doc.rows[0], big: null }] }).find("none");
  assert.deepEqual([none.half, none.minus, none.big], [null, null, null]);
  assert.throws(() => decodeMarks(load("marks.json")), /integer outside the TypeScript safe range/);
});

test("a Float32 is rounded once from the exact decimal (WIRE.md §5.1)", () => {
  const narrow = (token) => parseMarks(text("marks.json").replace(/"narrow": [0-9.e+-]+/, '"narrow": ' + token)).find("none").narrow;
  assert.equal(narrow("0.1"), Math.fround(0.1));
  assert.equal(narrow("1.000000059604644775390625"), 1);
  assert.equal(narrow("1.0000000596046447753906250000000001"), 1.0000001192092896);
  assert.equal(narrow("-1.0000000596046447753906250000000001"), -1.0000001192092896);
  assert.equal(narrow("3.4028235677973366e38"), 3.4028234663852886e38);
  assert.throws(() => narrow("3.40282356779733661637539395458142568448e38"), /overflows Float32/);
  const doc = load("marks.json");
  assert.equal(decodeMarks({ ...doc, rows: [{ ...doc.rows[0], big: null, narrow: 1.0000000596046447753906250000000001 }] }).find("none").narrow, 1);
});
