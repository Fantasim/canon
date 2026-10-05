import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseZone, decodeZone } from "./ref_lookup_empty_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./ref_lookup_empty_data/json/" + name, import.meta.url), "utf8");

test("a lookup over another package's empty table reads no cell (CODEGEN.md §5.10, log-2026-10-06 \"U5 re-verify\")", () => {
  const band = parseZone(text("zone.json")).band;
  assert.equal(band.base, 3);
  assert.deepEqual({ ...band.bonus }, {});
  const bad = JSON.parse(text("zone.json"));
  bad.value.band.$bonus = [];
  assert.throws(() => decodeZone(bad), /^Error: \/value\/band\/\$bonus: expected an object$/);
});
