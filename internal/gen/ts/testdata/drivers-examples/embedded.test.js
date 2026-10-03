import { test } from "node:test";
import * as assert from "node:assert/strict";
import { countries } from "./features/embedded/expected/embedded/out/countries.generated.js";

test("embedded: the data is compiled in as frozen literals (CODEGEN.md §2.1, §8.1)", () => {
  assert.equal(countries.length, 4);
  assert.deepEqual(countries.all.map((c) => c.code), ["FR", "DE", "BE", "CH"]);
  assert.equal(countries.find("CH").vat, 8.1);
  assert.equal(countries.find("CH").cards, false);
  assert.equal(countries.find("XX"), null);
  assert.ok(Object.isFrozen(countries.at(0)));
});
