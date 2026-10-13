import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { decodeStatuses, parseStatuses } from "./stable_data/features/a/a.js";

const text = readFileSync(new URL("./stable_data/json/statuses.json", import.meta.url), "utf8");

test("the data decoders return the lookups over the rows read (CODEGEN.md §5.9, DECISIONS 369)", () => {
  for (const statuses of [parseStatuses(text), decodeStatuses(JSON.parse(text))]) {
    assert.equal(statuses.findByCode(1).id, "open");
    assert.equal(statuses.findByTag("STALE").retired, true);
    assert.equal(statuses.findByCode(3), null);
    assert.equal(statuses.findByTag("closed"), null);
  }
});
