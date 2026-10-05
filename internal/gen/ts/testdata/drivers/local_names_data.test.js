import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseHolders } from "./local_names_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./local_names_data/json/" + name, import.meta.url), "utf8");

test("packages named p and raw read through their imports, not the decoders' own locals (CODEGEN.md §3.4)", () => {
  const h = parseHolders(text("holders.json")).find("h");
  assert.deepEqual(h.tones, ["loud", "soft"]);
  assert.deepEqual([...h.weights], [["soft", 3]]);
  assert.deepEqual({ ...h.note }, { kind: "big", tones: ["soft"] });
  assert.deepEqual(h.notes.map((n) => n.kind), ["small"]);
  assert.deepEqual({ ...h.weight }, { soft: 1, loud: 2 });
  const bad = JSON.parse(text("holders.json"));
  bad.rows[0].note.kind = "huge";
  assert.throws(() => parseHolders(JSON.stringify(bad)), /\/rows\/0\/note\/kind: unknown member/);
});
