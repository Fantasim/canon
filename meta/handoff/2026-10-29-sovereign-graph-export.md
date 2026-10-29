# Sovereign -> Canon: `canon graph`, the whole reference graph in one pass

**From:** Sovereign services/wiki (Louis). **Canon version:** 0.1.12. **Status:** open.

## Why

Sovereign is replacing its hand-written wiki with one generated from Resource. A wiki must answer
"how do I get X" and "what is X used for" for every entry. That means following references
across packages: a crafting recipe's components and results, a giftbox's contents, an event's
drops, a monster's drop table, an upgrade table's materials.

`canon refs` already knows all of this, labelled by role:

```
items:items.II_GEN_MAT_MOONSTONE is referenced 18 times
  items/catalog/craftingData.canon:407  value  items.catalog:craftingRecipes[26].components[0].id
  items/catalog/craftingData.canon:855  value  items.catalog:craftingRecipes[69].resultItems[0].id
  items/catalog/itemBoxesData.canon:6141  value  items.catalog:boxGiftboxes[3].items[0].item
  events/eventConfigData.canon:83  value  events:eventConfig.events[minister_env_custom_drop].drops[0].itemId
```

However, it answers one target per call, and each call re-analyses the project: about 32 s wall
(105 s CPU) on Resource. Sovereign has about 7,300 items, 985 movers, 600 NPCs and so on, so
looping `canon refs` would take days. `Project.Refs` (api/value.go) already runs `analyze` once
and then walks for one target. The ask is the same analysis, walked once, emitting every edge.

This is generic: any project with `ref` types has a reference graph. Possible uses beyond a wiki:
- a "where is this used" index for a studio;
- dead-entry detection (entries nothing references);
- impact review before a rename or retire;
- documentation generators.

Nothing in the ask is Sovereign-specific.

## Ask

### 1. API

```go
// Graph streams every node and every reference edge of the selected packages
// (all packages when none), from one analysis.
func (p *Project) Graph(ctx context.Context, o GraphOptions, fn func(GraphItem) error) error

type GraphOptions struct {
    Packages []string // empty = every package
    Kinds    []RefKind // which edge kinds; empty = value, key (see §3)
    Members  bool      // also edges to enum members (default false: they are numerous)
    Retired  bool      // include retired entries and edges to them (default false)
    Lang     string    // language for node titles (§2); empty = project default
}
```

`GraphItem` is exactly one of `*GraphNode` or `*GraphEdge`. Streaming matters because the
Resource graph will be large, likely more than a million edges with members on.

### 2. Nodes

One node per table entry, keyed-list element, and element of a root-level list:

| Field | Meaning |
|---|---|
| `id` | canonical path, e.g. `items:items.II_GEN_MAT_MOONSTONE`, `items.catalog:craftingRecipes[26]` |
| `package`, `root` | owning package and root (`craftingRecipes`) |
| `type` | the record type's qualified name |
| `title` | the target title as the view model already renders it (VIEWMODEL.md S8 / `Heading.Cells`), in `Lang`; empty when the type has no title |
| `retired` | bool |
| `file`, `line` | declaration span |

### 3. Edges

One edge per reference that `Refs` would report for the target. Same kinds (R7):
`value`, `key`, `code`, `view`, `check`, `layer`. Default is `value` and `key` only, because
the others are source-level noise for a data graph.

| Field | Meaning |
|---|---|
| `from` | the **owning node** of the referring value: root plus first subscript (`items.catalog:craftingRecipes[26]`), or the entry for a table (`items:items.II_X`); a root with no subscript owns itself (`events:eventConfig`) |
| `path` | the full canonical path of the referring value, as `Refs` prints it today |
| `pattern` | `path` with every list index and every key replaced by `*`: `items.catalog:craftingRecipes[*].components[*].id` |
| `to` | the target's canonical path (entry, keyed element, or member when `Members`) |
| `kind` | the `RefKind` |
| `computed` | true when the value is computed, not stated (R7) |
| `file`, `line` | span |

`pattern` is the important field. Hundreds of thousands of edges collapse into a few hundred
patterns, and those are what a person (or an agent) reviews and labels ("crafting: material",
"crafting: result", "giftbox: content"). The pattern must be stable for as long as the schema
is: renaming data does not change it, renaming a field does.

### 4. CLI

```
canon graph [packages…] [--kinds value,key] [--members] [--retired] [--lang fr]
            [--format jsonl|json] [--summary] [-o file]
```

- **`--format jsonl`** (default for `graph`): one `{"node":{…}}` or `{"edge":{…}}` object per
  line. Order: all nodes, then all edges, each sorted by canonical path. The output must be
  byte-identical across runs (goldens).
- **`--summary`**: no nodes or edges. Prints one row per `(pattern, from type, to type, kind)`
  with its count, sorted by pattern. Each row also gives up to 3 example `from → to` pairs. This
  is the small, reviewable view, and probably the most used form.

  ```
  items.catalog:craftingRecipes[*].components[*].id  CraftingRecipe → items.Item  value  2,184
      craftingRecipes[26] → II_GEN_MAT_MOONSTONE, …
  ```

### 5. Failure behaviour

Same rule as R7: a value that could hold a ref but cannot be computed makes `Graph` fail,
naming it, so a partial graph never looks complete. Optionally, `--partial` could keep going
and list those values as findings. That is your call.

## Acceptance

1. **Parity with `refs`.** For every target T, the edges with `to == T` and the default kinds
   equal `canon refs T` restricted to those kinds: same paths, same spans. This is a property
   test over `examples/` and an opt-in test on `testdata-real/` (sampled targets).
2. **`--summary` counts** sum to the edge count of the same run.
3. **Determinism.** Two runs give byte-identical output. Goldens live under `examples/**/expected/`.
4. **Performance on `testdata-real/`:** wall time ≤ 2× `canon check` on the same tree, and
   memory within the bounds of `make realdata-compile`.
5. **Docs.** CLI.md gets a §3.x, and spec/API.md gets `Graph` with its rules (owning node,
   pattern, order). Its diagnostics are added to ERRORS.md if any are new.

## Not asked

- No graph queries (paths, reachability) in Canon. Sovereign does those over the JSONL.
- No Sovereign vocabulary: no "obtain"/"use" meanings, no wiki notions. Those labels live in
  Resource, in a Sovereign package that names patterns (`Resource/Config/services/wiki/`).

## Current workaround

None that scales. A Go tool on the API could call `Refs` per target, but each call re-analyses
the project, which is the 32 s above.
