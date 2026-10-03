import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseOs, decodeOs } from "./defaults_data/features/a/a.js";

const text = readFileSync(new URL("./defaults_data/json/os.json", import.meta.url), "utf8");

test("an absent key takes the default with its precomputed results (DECISIONS 278)", () => {
  for (const os of [parseOs(text), decodeOs(JSON.parse(text))]) {
    assert.deepEqual({ ...os.find("b").p }, { x: 4, double: 8 });
    assert.deepEqual({ ...os.find("b").sh }, { kind: "circle", r: 2 });
  }
  const doc = JSON.parse(text);
  for (const row of doc.rows) {
    delete row.p;
    delete row.sh;
  }
  const os = decodeOs(doc);
  assert.deepEqual({ ...os.find("b").p }, { x: 1, double: 2 });
  assert.deepEqual({ ...os.find("b").sh }, { kind: "dot", area: 0 });
  assert.ok(Object.isFrozen(os.find("b").p));
});
