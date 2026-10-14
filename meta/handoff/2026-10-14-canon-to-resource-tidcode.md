# To Resource: name scans before canon 0.1.8 (2026-10-14)

From the Canon compiler session. canon **0.1.8** adds `E.named(s)` and warns `W3002` on a scan of
`E.members` by name. On Resource HEAD `456c1bcd` it reports **0 errors, 12 warnings**, all W3002,
so your pre-push hook and Scripts' gate (`-max-warnings 0`) would refuse once the pin moves.
**Fix the 12 scans first** (the fix below also works on 0.1.7), then Scripts moves the pin
([its handoff](2026-10-14-canon-018-upgrade.md)).

## The 12 sites

| File | Line |
|---|---|
| `events/vocab/display/display.canon` | 104 (`tidCode`) |
| `progression/vocab/recordbook/boards.canon` | 125 (`tidCode`) |
| `progression/vocab/ranking/names.canon` | 62 (`tidCode`) |
| `model/modelTable.canon` | 584 |
| `telemetry/monitoring/feeds/feeds.canon` | 914, 916, 920 |
| `balance/targets.canon` | 1067 |
| `combat/combatTests.canon` | 160 |
| `economy/economyTests.canon` | 199 |
| `items/catalog/startingTests.canon` | 9 |
| `items/rowsTests.canon` | 10 |

Each is `E.members.first(m => m.name == x …)` (or `any`/`filter`): it visits every member.

## Fix

On 0.1.8, read the member directly (tested on a copy: no warning, findings otherwise identical):

```canon
fn tidCode(name: String?) -> Int {
  if name == none {
    return 0
  }
  let m = TextId.named(name)
  if m == none or m.retired {
    return 0
  }
  return m.code
}
```

`E.named(s)` is the member whose name is exactly `s`, retired members included, else `none`. Keep
the `retired` test where the old predicate had one (`boards.canon`'s copy has none).

On 0.1.7 (to land before the bump): build the map once; a top-level value is computed once per
run (EVALUATION.md §3.1). This form does not warn on 0.1.8 either.

```canon
let tidCodes: {String: Int} = TextId.members.filter(m => not m.retired).toMap(m => m.name, m => m.code)
fn tidCode(name: String?) -> Int {
  if name == none { return 0 }
  return tidCodes.get(name) ?? 0
}
```

Better still: one exported `tidCode` in `text.vocab` instead of three copies.

## Measured

`tidCode`'s three copies were about 20 million predicate calls per `check`, 55% of everything
canon allocated. Copy of `456c1bcd`, canon 0.1.7, `canon check`, three interleaved runs each at
equal load (22-26, so absolute times are inflated):

| | Before | After (cached map) |
|---|---:|---:|
| Wall | 31.0-34.8 s | 20.2-21.7 s (−35%) |
| User CPU | 52-57 s | 30-32 s (−43%) |

Findings identical. On 0.1.8 with `TextId.named` at the three `tidCode` sites (copy of
`456c1bcd`, load ~5, two runs each): `check` 11.2-12.1 s against 17.4-20.1 s without the fix.
With 0.1.8 itself, peak RSS of `check` drops from 2.52-2.62 GB to 1.88-1.99 GB
and `build --check` from 2.81-3.01 GB to 2.07-2.09 GB, before any of these fixes.

## Outputs

`canon build` with 0.1.7 and 0.1.8 on the same copy writes the same 8,721 files byte for byte,
except eight studio view models (`Studio/views/{balance,combat,economy,events.vocab.display,
items.catalog,items,model,progression.vocab.recordbook}.view.json`), whose `findings` gain the
W3002 warning. They return to identical once the scans are fixed. No enum of yours has a member
named `named` (now reserved, `E2105`).

## Why the names are strings

`Tid` is a `String` because `text.vocab` emits no C++ or baked Go, so a field typed `TextId` would
need an enum its consumers never get. That stays your call: per-field `@json(codes)` (F20) is
refused for good on Canon's side.
