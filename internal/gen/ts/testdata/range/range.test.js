import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";

// CODEGEN.md §5.15: each run of range-cases.json calls a parse function of a generated module on a text and prints "ok", or "error " and the decoder's message.
const runs = JSON.parse(readFileSync(new URL("./range-cases.json", import.meta.url), "utf8"));
const modules = new Map();

for (const run of runs) {
  test(run.module + " " + run.name, async () => {
    if (!modules.has(run.module)) modules.set(run.module, await import("./" + run.module));
    let got = "ok";
    try {
      modules.get(run.module)[run.fn](run.text);
    } catch (e) {
      got = "error " + e.message;
    }
    assert.equal(got, run.want);
  });
}
