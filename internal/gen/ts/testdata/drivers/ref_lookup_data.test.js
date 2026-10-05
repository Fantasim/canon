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
  assert.throws(() => decodeZone(bad), /^Error: \/value\/band\/\$next\/low: no entry nope$/);
  const missing = JSON.parse(text("zone.json"));
  delete missing.value.band.$bonus.high;
  assert.throws(() => decodeZone(missing), /^Error: \/value\/band\/\$bonus\/high: expected an integer$/);
});
