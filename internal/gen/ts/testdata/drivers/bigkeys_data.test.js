import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseRs } from "./bigkeys_data/features/a/a.js";

const text = readFileSync(new URL("./bigkeys_data/json/rs.json", import.meta.url), "utf8");

test("parse reads bigint refs and keys past 2^53 exactly, as baked writes them (DECISIONS 278)", () => {
  const rs = parseRs(text);
  const a = rs.find(9007199254740993n);
  assert.equal(a.n, 9007199254740993n);
  assert.equal(a.next, 9007199254740993n);
  assert.equal(rs.find(a.next), a);
  assert.equal(rs.find(rs.find(1n).next), a);
  assert.deepEqual([...a.m], [[9007199254740995n, 1n]]);
  assert.deepEqual([...a.l], [9007199254740997n]);
  assert.equal(a.mx, 9007199254740999n);
  assert.deepEqual([...a.w], [[1n, 2]]);
  assert.equal(rs.find(1n).mx, null);
  assert.throws(() => parseRs(text.replace('"9007199254740995"', '"9223372036854775808"')), /integer out of range/);
  assert.throws(() => parseRs(text.replace('"9007199254740995"', '"1.5"')), /expected a decimal integer key/);
});
