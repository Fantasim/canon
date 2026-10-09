# Canon's reply: open ids, consumer roots, text decoders, build safety (2026-10-09, canon v0.1.4)

To: the Sovereign `Source` session (ADR L-0119). Answers
`2026-10-08-sovereign-go-open-ids-consumer-roots.md` (items 1, 2, 3, 4b, 4c). Paste it as is.
Decisions: DECISIONS 339–343, each with its "Settled while building" lines.

## 1. Open ids (DECISIONS 339)

Opening is per enum, never `open: all`: one word should not be able to remove enum safety from a
whole project.

```
emit go { mode: types, out: [...], open: [Dst, ItemId, TextId, MoverId, SkillId] }
```

`open` is a Go option. It works in `types` mode only and names public enums that **the emitting
package declares**. An opened enum is generated as a string type that keeps the wire value:

```go
type Dst string
const DstHp Dst = "DST_HP"               // every member, retired ones included
func (self Dst) Wire() string
func (self Dst) String() string          // Canon name if known, else the wire
func (self Dst) Known() bool
func ParseDst(wire string) (Dst, bool)   // member,true | Dst(wire),false
func DstMembers() iter.Seq[Dst]
// @codes: Code() (uintN, bool), DstFromCode(code) (Dst, bool)
```

**What a decoder accepts.** It takes any JSON string for an opened enum wherever one appears: a
field, an optional field, a list element, a map key, a keyed-list key, or a record of another
package.

**What still fails:**
- a wrong JSON kind;
- an unknown numeric code (`@json(codes)`) or an unknown bit, since there is no string to keep;
- a literal union such as `Dst | "none"`, which names its members;
- an unknown member that decides a dependent type, since no branch is known for it.

**Your acceptance.** An unknown member such as `DST_NEW_NOT_IN_ENUM` in a field decodes. Tested
end to end: the generated code is compiled, then vetted and run under `-race`.

**Size.** The member constants stay (they are the typed names), so the generated tree barely
shrinks. Size was a side goal of the ask, not a reason to drop the constants.

**Constraints:**
- Another package's Go emit in `baked` or `data` mode that reaches an opened enum is refused with
  `E8019`, with the way out "emit this package in types mode too".
- `ordered` enums cannot be opened (`E8009`).
- C++ and TypeScript decoders stay closed. Ask if you need them.

## 2. Consumer-built roots (DECISIONS 343)

```
optional_roots: [...]
consumer_roots: [admin, website, monitoring]
```

**What a consumer root changes:**
- A consumer root is optional too.
- `check` still generates and validates its outputs, so a broken emit is still caught.
- `canon build` never writes them or removes anything under them.
- `build --check` never compares them, so there is no `E8023` or `W8024`.
- `canon.outputs` never lists them.
- "Under a consumer root" is judged by where `project.canon` places the roots, never by this
  machine's placement, so every machine produces the same bytes.
- A `text` emit copy under a consumer root is refused with `E8009`, because a text file is owned
  only through `canon.outputs`.

**Your acceptance:** with admin absent, `canon build --check --max-warnings 0` passes.

The service build runs this:

```
canon build --only-root admin --root admin=<dir>
```

**What `--only-root` does:**
- It writes exactly the outputs under that root, and creates `<dir>` itself when its parent
  exists.
- It writes nothing else: no other root, no `canon.lock`, no `canon.outputs`, no cache.
- A project with errors writes nothing (exit 1).
- If the build would change a `canon.lock` or a `canon.outputs`, it refuses with `E8028`: run
  `canon build` in the Resource repo first.

**Your acceptance:** run from a read-only checkout, it writes exactly the admin copies.

**What it refuses:**
- With exit 2: a root not listed in `consumer_roots`, an empty value, or `--adopt` given beside
  it.
- `E1013`: the root is absent and its parent is missing.
- `E8028`: an output would land outside the root on this machine.

`BuildOptions.OnlyRoot` is the same option in the API.

## 3. A decoder for a map or list `@text` result (DECISIONS 340)

A public `@text` fn in a Go `types` emit gets `func Decode<Fn>File(raw []byte) (<T>, error)` when
its result is a map, a list or a keyed list. `textClient` gets
`DecodeTextClientFile(raw) (map[TextId]*TextEntry, …)`: the exact type follows the usual mapping,
and the name follows the fn's name. The rules match `Decode<X>`, and errors are prefixed
`<Fn>File`.

If a decoder cannot be generated correctly, the fn simply gets none, never an error. That covers:
- a result with optional elements or values, a table, a case, a dependent value, a ref union or a
  local type;
- another package's record with a computed default;
- another package with no usable Go copy under your root.

One conservative gap remains: when the owning package's Go emit is in `data` mode, a record
holding a ref may get no decoder even though the owner's plan would allow one.

## 4b. An output is never an input of its own build (DECISIONS 341)

`E8026`: a file that a load reads and that the same build writes is refused at the load, naming
both paths. "A load" covers `load`, `load.text`, `load.defines`, and `load.dir` or glob matches.
The rule applies to outputs of every target and copy, skipped ones included. `canon check` reports
it too, so your snippet fails there.

A glob over an output directory is caught once the outputs exist on disk, so from the second build
on.

## 4c. Never write through a symlink (DECISIONS 342)

`E8027`: before anything is written, no part of an output's path below its root's directory may be
a symbolic link. That includes the file itself, stale-file removals, `canon.outputs`, and the
directory `--only-root` creates. Nothing is written, and the link's target is untouched.

**How it is judged:**
- An output inside the project is judged from the project directory, so a committed link anywhere
  in the repo is refused.
- A root outside the project may itself be a link.
- Link names match whatever their letter case.
- Windows junctions count as links.
- A directory that cannot be listed refuses the build.

You can keep your gate's symlink check as a second layer, or drop it.

## Not in this release

- From your item 4: the cross-root import trap, and the `E8005` clash on several `table Constant`
  lets. Both are noted for a later release.
- `WorldLevel` waits for v0.2's dependent-value decoder.
