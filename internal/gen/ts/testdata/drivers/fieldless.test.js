import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseHs } from "./fieldless/features/a/a.js";

test("a fieldless case with an export fn is read with its stored result (CODEGEN.md §5.5)", () => {
  const hs = parseHs(readFileSync(new URL("./fieldless/json/hs.json", import.meta.url), "utf8"));
  assert.deepEqual({ ...hs.find("a").s }, { kind: "point", atOrigin: true });
  assert.deepEqual({ ...hs.find("b").s }, { kind: "circle", r: 1 });
});
