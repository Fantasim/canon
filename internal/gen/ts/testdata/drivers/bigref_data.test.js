import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseH } from "./bigref_data/features/a/a.js";

const text = readFileSync(new URL("./bigref_data/json/h.json", import.meta.url), "utf8");

test("parse reads a ref in its target key's form beside @ts(bigint) positions (DECISIONS 279(a))", () => {
  const h = parseH(text);
  assert.deepEqual([...h.m], [[1, 9007199254740995n]]);
  assert.deepEqual([...h.byBig], [[9007199254740997n, 2]]);
  assert.equal(h.b, 9007199254740993n);
  assert.deepEqual([...h.bk], [[9007199254740993n, 1]]);
});
