import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseZone, decodeZone } from "./ref_lookup_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./ref_lookup_data/json/" + name, import.meta.url), "utf8");

test("a lookup over another package's table reads its cells by the owner's ids (CODEGEN.md §5.10, log-2026-10-06 \"U5 + 320 done\" G1)", () => {
  const band = parseZone(text("zone.json")).band;
  assert.deepEqual({ ...band.bonus }, { low: 4, high: 5 });
  assert.deepEqual({ ...band.next }, { low: "high", high: "low" });
  assert.deepEqual(Object.keys(band.bonus), ["low", "high"]);
});

test("a bad or missing cell fails at its path", () => {
  const bad = JSON.parse(text("zone.json"));
  bad.value.band.$next.low = "nope";
  assert.throws(() => decodeZone(bad), /^Error: zone\.json: value\.band\.\$next\.low: no entry nope$/);
  const missing = JSON.parse(text("zone.json"));
  delete missing.value.band.$bonus.high;
  assert.throws(() => decodeZone(missing), /^Error: zone\.json: value\.band\.\$bonus\.high: missing$/);
});

test("a null cell, or a null at a level, fails `null` at its path, except a cell of an optional result, which is none (CODEGEN.md §5.13, WIRE.md §5.11)", () => {
  const cellNull = JSON.parse(text("zone.json"));
  cellNull.value.band.$bonus.high = null;
  assert.throws(() => decodeZone(cellNull), /^Error: zone\.json: value\.band\.\$bonus\.high: null$/);
  const levelNull = JSON.parse(text("zone.json"));
  levelNull.value.band.$gated.low = null;
  assert.throws(() => decodeZone(levelNull), /^Error: zone\.json: value\.band\.\$gated\.low: null$/);
  const absentLevel = JSON.parse(text("zone.json"));
  delete absentLevel.value.band.$gated.high;
  assert.throws(() => decodeZone(absentLevel), /^Error: zone\.json: value\.band\.\$gated\.high: missing$/);
  const noneCell = JSON.parse(text("zone.json"));
  noneCell.value.band.$maybe.low = null;
  assert.equal(decodeZone(noneCell).band.maybe.low, null);
});

test("a stored result of an optional type may be absent or null: none, as the Go and C++ loaders read it (WIRE.md §5.4, §5.11)", () => {
  const given = JSON.parse(text("zone.json"));
  assert.equal(decodeZone(given).band.extra, null);
  given.value.band.$extra = 5;
  assert.equal(decodeZone(given).band.extra, 5);
  const absent = JSON.parse(text("zone.json"));
  delete absent.value.band.$extra;
  assert.equal(decodeZone(absent).band.extra, null);
  const isNull = JSON.parse(text("zone.json"));
  isNull.value.band.$extra = null;
  assert.equal(decodeZone(isNull).band.extra, null);
});

test("the envelope's value is required: absent is `missing`, null is `null` (CODEGEN.md §5.13)", () => {
  const absent = JSON.parse(text("zone.json"));
  delete absent.value;
  assert.throws(() => decodeZone(absent), /^Error: zone\.json: value: missing$/);
  const isNull = JSON.parse(text("zone.json"));
  isNull.value = null;
  assert.throws(() => decodeZone(isNull), /^Error: zone\.json: value: null$/);
});
