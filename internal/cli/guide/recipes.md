# Recipes

Run against the project of `canon guide data` (or `layers`); request files are in `req/`.

## Add an entry

```sh data
canon explain shop:items --depth 1
canon edit req/add-mace.json
canon check
```

```json req/add-mace.json
{"ops": [{"op": "addEntry", "path": "shop:items", "key": "mace", "source": "{ name: \"Mace\", kind: weapon, price: 80 }"}]}
```

The new file goes where the table's `@files` template says. On a stable table the key is
appended to `canon.lock` and is permanent (`canon guide cli`, undo). On a keyed list use `add`
(the key is a field of the value); on a map `addEntry`.

## Change a value safely

```sh data
canon explain 'shop:items.axe.price' --format json
canon edit req/revision.json
canon edit req/set-price.json
canon check
```

```json req/revision.json
{"ops": []}
```

```json req/set-price.json
{"ops": [{"op": "set", "path": "shop:items.axe.price", "value": 130}]}
```

`{"ops": []}` prints the current `revision`; put it in `"base"` when other writers may be
active. A value set by an active layer needs `--edit-layer <layer>`; a computed one is edited at
its source (`explain` shows it). Save the printed `undo` object as a file to revert.

## Add a field to a record on the wire

Add it with a default or as optional, so every literal and data file stays valid (a new
required field is `E3302` in every entry): `bundle: Int(1..) = 1 @json("nBundle")`. Then
`canon fmt`, `canon check`, `canon build`. The `$schema` fingerprint of emitted data changes:
`data`-mode runtimes refuse old data until rebuilt. Deploy code and data together.

## Rename a field (or type, function, let)

```sh data
canon rename shop:Item.price cost --dry-run
canon rename shop:Item.price cost
canon check
canon build
```

A field read from or written to JSON keeps its wire name (`@json("price")` is added when it had
none). Undo: the printed `undo` request, fed to `canon edit`. An entry key (non-stable) is data:
`{"op": "rename", "path": "shop:ranks[major]", "key": "colonel"}` moves every ref and map key.

## Retire a stable id

```sh data
canon refs shop:items.axe
canon edit req/retire.json
```

```json req/retire.json
{"ops": [{"op": "set", "path": "shop:starter[private]", "source": "[bow]"}, {"op": "set", "path": "shop:starter[captain]", "source": "[bow]"}, {"op": "retire", "path": "shop:items.axe"}]}
```

The edit writes the `retired` lock line. Any stored value whose `ref` points to the retired
one is `E3502` (new use of a retired entry), so repoint them all first, as the ops above do;
only a retired entry and a `past ref T` slot (`canon guide types`) may keep the key. Never delete,
rename or reuse a stable id (`E6001`, `E6002`). A `@codes` member is retired the same way (`Kind.old`).

## Add a check

Add `check cheapPotion: kind != potion or price <= 500 at price else "{name}: over 500"` to
the record, and `expect Item { name: "Elixir", kind: potion, price: 900 } fails cheapPotion` to
a `test` (`canon guide logic`). Then `canon fmt`, `canon check` (existing data may now fail: fix it or soften to `warn`),
`canon test --run potions`.

## Load a JSON file and fix its findings

Write the type, then `let limits: Limits = load("data/limits.json")`.

```sh data
canon check --format json shop
canon fmt --json-sources
canon edit req/fix.json
```

```json req/fix.json
{"ops": [{"op": "set", "path": "shop:limits.players", "value": 120}]}
```

`--json-sources` normalizes loaded JSON once, before `canon edit` writes it. A wrong rule is
fixed in the type: wire names (`@json("...")`), `T?` with `@json(none: ...)`, a default,
`partial: true` for keys the type does not model.

## Add a translation

Create `shop/shop.fr.canon` as in `canon guide views-i18n` (`fr` in `languages`), then
`canon check`: unknown keys are `E1702`, untranslated texts one `W1701` per package.

## Change a value for one environment

```sh layers
canon edit req/port.json --edit-layer staging
canon check --layer staging
canon explain 'app:config.server.port' --layer staging
```

```json req/port.json
{"ops": [{"op": "set", "path": "app:config.server.port", "value": 9100}]}
```

The edit writes `app/staging.layer.canon`, created when missing.

## Share feature flags with C++

Keep the flags in Canon and render the header the C++ build includes: one source of truth, and
no header to parse back (`load.defines` reads constants, not `#if` blocks). A file of a system
that is off is a `@text` fn returning `none` (`canon guide emit`).

```canon flags/flags.canon
/// Feature flags shared with the C++ build.
package flags

/// The arena system.
const ARENA = true

/// The header C++ includes: one `#define` per system that is on.
@text("Sys_Features.h")
export fn header() -> String {
  return "#pragma once\n{if ARENA { "#define SYS_ARENA\n" } else { "" }}"
}

emit text { out: "include" }
```

```text out/flags/include/Sys_Features.h
#pragma once
#define SYS_ARENA
```
