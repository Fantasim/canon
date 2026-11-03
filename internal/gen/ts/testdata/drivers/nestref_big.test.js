import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as bgb from "./nestref_big/features/bgb/bgb.js";
import * as bgd from "./nestref_big/features/bgd/bgd.js";

const big = 9007199254740993n;

test("baked: a ref two and three deep to a @ts(bigint) leaf is a bigint, exact past 2^53 (DECISIONS 278, 456)", () => {
  assert.equal(bgb.h.two, big);
  assert.equal(bgb.h.three, big);
  assert.deepEqual([...bgb.h.l], [5n, big]);
  assert.deepEqual([...bgb.h.m], [[big, 1]]);
  assert.equal(bgb.h.plain, 7);
  assert.notEqual(bgb.b2.find(bgb.h.two), null);
  assert.notEqual(bgb.b3.find(bgb.h.three), null);
  assert.deepEqual(bgb.b3.all.map((r) => r.k), [big, 5n]);
});

test("data: the file's integers read as bigint down the chain (DECISIONS 278, 456)", () => {
  const text = (name) => readFileSync(new URL("./nestref_big/json/" + name, import.meta.url), "utf8");
  const h = bgd.parseH(text("h.json"));
  assert.equal(h.two, big);
  assert.equal(h.three, big);
  assert.deepEqual([...h.l], [5n, big]);
  assert.deepEqual([...h.m], [[big, 1]]);
  assert.equal(h.plain, 7);
  assert.equal(h.ev, big);
  assert.deepEqual({ ...h.param }, { branch: "count", value: 3 });
  assert.deepEqual(bgd.parseB3(text("b3.json")).all.map((r) => r.k), [big, 5n]);
  assert.notEqual(bgd.parseB2(text("b2.json")).find(h.two), null);
});
