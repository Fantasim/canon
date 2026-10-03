import { test } from "node:test";
import * as assert from "node:assert/strict";
import { parseH } from "./bigref_types/features/a/a.js";

test("a types-mode decoder reads a ref in its target key's form beside @ts(bigint) positions (DECISIONS 279(a))", () => {
  const h = parseH('{"m": {"1": 9007199254740995}, "byBig": {"9007199254740997": 2}, "b": 9007199254740993, "bk": {"9007199254740993": 1}}');
  assert.deepEqual([...h.m], [[1, 9007199254740995n]]);
  assert.deepEqual([...h.byBig], [[9007199254740997n, 2]]);
  assert.equal(h.b, 9007199254740993n);
  assert.deepEqual([...h.bk], [[9007199254740993n, 1]]);
});
