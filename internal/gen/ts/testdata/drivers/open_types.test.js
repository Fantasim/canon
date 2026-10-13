import { test } from "node:test";
import * as assert from "node:assert/strict";
import {
  DstMembers, DstNames, DstIndex, DstCodes, isDstKnown, dstName, toneName, isToneKnown, ToneMembers,
  decodeRule, parseRule, decodeRulesFile, parseRulesFile, decodeTonesFile, parseTonesFile, parseNamedFile, parseMaybeFile,
} from "./open_types/features/a/a.js";

const rule = (extra) => ({ dst: "DST_HP", tone: "calm", grade: 1, goal: "kill", target: "wolf", ...extra });

test("an opened enum keeps the closed tables and adds is<E>Known and <e>Name (CODEGEN.md §5.2, DECISIONS 362)", () => {
  assert.deepEqual([...DstMembers], ["DST_HP", "DST_MP", "DST_OLD"]);
  assert.equal(DstNames.DST_MP, "DST_MP");
  assert.equal(DstIndex.DST_OLD, 2);
  assert.equal(DstCodes.DST_OLD, 3);
  assert.equal(isDstKnown("DST_HP"), true);
  assert.equal(isDstKnown("DST_NEW"), false);
  assert.equal(isDstKnown("toString"), false);
  assert.equal(dstName("DST_HP"), "DST_HP");
  assert.equal(dstName("DST_NEW"), "DST_NEW");
  assert.equal(toneName("LOUD"), "loud");
  assert.equal(toneName("shouting"), "shouting");
  assert.deepEqual([...ToneMembers], ["calm", "LOUD"]);
  assert.equal(isToneKnown("calm"), true);
});

test("a decoder reads any string for an opened enum: field, element, map key (CODEGEN.md §5.2, §5.13)", () => {
  const r = decodeRule(rule({ dst: "DST_NEW", tone: "weird", tones: ["calm", "odd"], byTone: { calm: 1, other: 2 } }));
  assert.equal(r.dst, "DST_NEW");
  assert.equal(r.tone, "weird");
  assert.deepEqual([...r.tones], ["calm", "odd"]);
  assert.deepEqual([...r.byTone], [["calm", 1], ["other", 2]]);
  assert.equal(parseRule('{"dst": "X", "tone": "Y", "grade": 2, "goal": "kill", "target": "w"}').dst, "X");
  assert.throws(() => decodeRule(rule({ dst: 1 })), new Error("Rule: $.dst: expected a string"));
  assert.throws(() => decodeRule(rule({ tones: [1] })), new Error("Rule: $.tones[0]: expected a string"));
});

test("a literal union over an opened enum stays closed: its members' wires and its literals (CODEGEN.md §5.2, §5.13, DECISIONS 362)", () => {
  assert.equal(decodeRule(rule()).mood, "any");
  for (const mood of ["calm", "LOUD", "any"]) assert.equal(decodeRule(rule({ mood })).mood, mood);
  assert.throws(() => decodeRule(rule({ mood: "weird" })), new Error("Rule: $.mood: unknown value weird"));
  assert.throws(() => decodeRule(rule({ mood: "loud" })), new Error("Rule: $.mood: unknown value loud"));
  assert.throws(() => decodeRule(rule({ mood: 2 })), new Error("Rule: $.mood: expected a string"));
});

test("a numeric wire and a dependent discriminant stay closed (CODEGEN.md §5.2)", () => {
  assert.equal(decodeRule(rule({ grade: 2 })).grade, "high");
  assert.throws(() => decodeRule(rule({ grade: 9 })), new Error("Rule: $.grade: unknown code"));
  assert.throws(() => decodeRule(rule({ grade: "high" })), new Error("Rule: $.grade: unknown code"));
  assert.throws(() => decodeRule(rule({ goal: "dance" })), new Error("Rule: $.target: no branch for this value"));
  assert.equal(decodeRule(rule({ goal: "dance", target: undefined })).goal, "dance");
  assert.deepEqual({ ...decodeRule(rule()).target }, { branch: "kill", value: "wolf" });
});

test("decode<Fn>File and parse<Fn>File read a decoded @text result, deep-frozen (CODEGEN.md §5.13, §8.1, DECISIONS 363)", () => {
  const rules = parseRulesFile(JSON.stringify([rule(), rule({ dst: "Z" })]));
  assert.equal(rules.length, 2);
  assert.equal(rules[1].dst, "Z");
  assert.ok(Object.isFrozen(rules));
  assert.ok(Object.isFrozen(rules[0]));
  assert.equal(decodeRulesFile([rule()])[0].tone, "calm");
  const tones = parseTonesFile('{"zz": 1, "calm": 2, "10": 3}');
  assert.deepEqual([...tones], [["zz", 1], ["calm", 2], ["10", 3]]);
  assert.throws(() => tones.set("a", 1), TypeError);
  assert.deepEqual(parseNamedFile('[{"name": "a", "dst": "Q"}]').map((n) => n.dst), ["Q"]);
  assert.equal(parseMaybeFile("[]").length, 0);
  assert.equal(decodeTonesFile({ a: 1 }).get("a"), 1);
});

test("a text decoder's errors name <Fn>File behind a $ path (CODEGEN.md §5.13, DECISIONS 363)", () => {
  assert.throws(() => decodeRulesFile({}), new Error("RulesFile: $: expected an array"));
  assert.throws(() => decodeRulesFile([rule(), rule({ dst: 1 })]), new Error("RulesFile: $[1].dst: expected a string"));
  assert.throws(() => decodeTonesFile({ a: "x" }), new Error("TonesFile: $.a: expected an integer"));
  assert.throws(() => parseTonesFile('{"a": 1, "a": 2}'), new Error("TonesFile: $.a: duplicate key"));
  assert.throws(() => parseRulesFile("[1,"), new Error("RulesFile: invalid JSON at byte 3"));
  assert.throws(() => parseNamedFile("{"), new Error("NamedFile: invalid JSON at byte 1"));
});

test("a valid number token is never a syntax error (CODEGEN.md §5.13)", () => {
  assert.throws(() => parseRule('{"dst": "A", "tone": "t", "grade": 1, "goal": "visit", "f": 1e400}'), new Error("Rule: $.f: number out of range"));
  assert.throws(() => parseRule('{"dst": "A", "tone": "t", "grade": 1, "goal": "visit", "f": -1e400}'), new Error("Rule: $.f: number out of range"));
  assert.throws(() => parseRule('{"dst": "A", "tone": "t", "grade": 1, "goal": "visit", "f": "x"}'), new Error("Rule: $.f: expected a number"));
  assert.equal(parseRule('{"dst": "A", "tone": "t", "grade": 1, "goal": "visit", "ignored": 1e400, "f": 1.5}').f, 1.5);
  assert.throws(() => parseRule("1e400"), new Error("Rule: $: expected an object"));
  assert.throws(() => decodeRule({ ...rule(), f: 1e400 }), new Error("Rule: $.f: number out of range"));
});

test("within one string the failure that becomes certain first is reported, a tie goes to the surrogate (CODEGEN.md §5.13)", () => {
  const at = (body) => () => parseRule('{"a": "' + body + '"}');
  assert.throws(at("\\ud800\ud800"), new Error("Rule: $.a: unpaired surrogate"));
  assert.throws(at("\\ud800\udc00x"), new Error("Rule: $.a: unpaired surrogate"));
  assert.throws(at("\\udc00\ud800"), new Error("Rule: $.a: unpaired surrogate"));
  assert.throws(at("\ud800\\udc00"), new Error("Rule: $.a: not UTF-8"));
  assert.throws(() => parseRule('{"a": "\\udc'), new Error("Rule: invalid JSON at byte 11"));
  assert.throws(() => parseRule('{"a": "\\udc00\ud800'), new Error("Rule: $.a: unpaired surrogate"));
});
