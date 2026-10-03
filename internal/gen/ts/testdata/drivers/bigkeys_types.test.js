import { test } from "node:test";
import * as assert from "node:assert/strict";
import { parseR } from "./bigkeys_types/features/a/a.js";

test("types mode: a bigint ref and bigint map keys (DECISIONS 278)", () => {
  const r = parseR('{"n": 9007199254740993, "next": 9007199254740993, "m": {"9007199254740995": 1}, "w": {"9007199254740993": 2}}');
  assert.equal(r.next, 9007199254740993n);
  assert.deepEqual([...r.m], [[9007199254740995n, 1n]]);
  assert.deepEqual([...r.w], [[9007199254740993n, 2]]);
});
