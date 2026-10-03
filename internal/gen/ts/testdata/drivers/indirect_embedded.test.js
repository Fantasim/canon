import { test } from "node:test";
import * as assert from "node:assert/strict";
import { badges, one } from "./indirect_embedded/features/a/a.js";

test("a literal writes a type of a package reached only through another's types (DECISIONS 279(b))", () => {
  assert.equal(one.tags.size, 0);
  assert.deepEqual({ ...badges.find("y").tags.get("k") }, { t: "q", big: 9007199254740993n });
  assert.throws(() => badges.find("y").tags.set("z", { t: "z", big: 0n }));
});
