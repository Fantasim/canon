# Stable tables

An id that leaves the project (a database column, a save file, a network packet) must never
change meaning. A `stable table` makes its keys permanent; a `@stable` field makes a number or
a string on each entry permanent. `canon.lock` remembers both, and `canon build` and
`canon edit` refuse to break it (LOCK.md §1). Everything else (plain tables, plain enums,
variants) may change freely.

## Stable tables

What: a table whose entry keys are forever. Add entries, retire them, never remove, rename or
reuse a key.
When: the key is stored outside the project, or other data refers to it by key and must keep
working. Not for tuning data that only the project reads: use `table`.

```canon blog/blog.canon
/// Post lifecycle statuses, stored in a database by number.
package blog

/// One status of a post.
record Status {
  /// The number stored in the database. It is never reused.
  code: UInt16 @stable
  /// Shown label.
  label: String(1..)
  /// Statuses a post may move to.
  next: [ref statuses] = []
}

/// Every status. The keys are permanent: retire a status, never remove or rename it.
let statuses: stable table Status = {
  draft { code: 1, label: "Draft", next: [review] }
  review { code: 2, label: "In review", next: [published] }
  published { code: 3, label: "Published" }
  retired hidden { code: 4, label: "Hidden" }
}
```

A stable table may also grow by entry files, one entry per file (`canon guide data`):

```canon blog/archived.canon
package blog

entry statuses.archived {
  code: 5
  label: "Archived"
}
```

`canon build` writes `canon.lock` next to the package: one line per key and per `@stable`
value ever seen, sorted. Commit it.

```text out/blog/canon.lock
# canon.lock v1
field  blog.statuses.code  1  draft
field  blog.statuses.code  2  review
field  blog.statuses.code  3  published
field  blog.statuses.code  4  hidden
field  blog.statuses.code  5  archived
table  blog.statuses  archived
table  blog.statuses  draft
table  blog.statuses  hidden  retired
table  blog.statuses  published
table  blog.statuses  review
```

- `stable table T` is the whole type of a top-level `let`, public or `local`.
- Retired entries stay in the source, the data files, the lock and the generated id type. The
  table's iteration keeps them, `.active()` skips them, a `ref` to one is `E3502` outside a
  retired entry or a `past ref` slot (`canon guide data`, `canon guide recipes`).
- The element may not declare `id` or `retired` (`E2105`): the key is the entry's `.id`.
- A new key needs nothing: `canon build` appends it to the lock.

## `@stable` fields

What: a field of the element whose value on each entry is locked too: the number the database
stores, a short code a client sends. The key says which entry; the `@stable` field is the
persistent name for it.
When: an id whose stored form is not the key. One `@stable` field per stored form.

- The field is an integer type or `String`, not optional (`E6003` otherwise), on a record that
  is the element of a stable table (`E6003` on a plain table: the lock cannot hold it).
- Its values are unique across all entries, retired ones included (`E3102`). A value never
  changes (`E6002`) and is never reused by another entry (`E6002`, `value 1 of code belongs to
  draft`).
- The lock holds a `field` line per entry; a `String` value is JSON-quoted.
- Mistake: storing the generated id enum's number (below). It is the entry's position in this
  build. Store the `@stable` value, or the key.

## What is generated

Go, C++ and TypeScript get the table, one id type with a member per key (retired included), and a
lookup per `@stable` field (names from the field: `code` gives `FindByCode`). The sketch below is
the baked form of `blog`; `data` mode reads the rows from a file.

```text
// Go
type StatusID uint8                  // StatusIDDraft = 0, ..., StatusIDArchived = 4
func (self *Statuses) Find(key string) (*Status, bool)
func (self *Statuses) Get(id StatusID) *Status
func (self *Statuses) FindByCode(code uint16) (*Status, bool)
func (self *Status) Code() uint16
func (self *Status) Retired() bool

// C++
enum class StatusId : uint8_t { draft, review, published, hidden, archived };
const Status* Find(std::string_view id) const;
const Status* FindByCode(uint16_t code) const;
uint16_t GetCode() const;
bool GetRetired() const;

// TypeScript
export type StatusId = "draft" | "review" | "published" | "hidden" | "archived";
export const statuses: CanonTable<StatusId, CanonRow<Status, StatusId>>;
```

- `FindByCode` finds retired entries too: stored data still names them. Check `Retired()`.
  TypeScript has the rows and their `code`, no lookup function.
- The id enum value is the entry's position (literal first, then entry files by path). Adding an
  entry may change it: never persist it; persist the key or the `@stable` value.
- In `data` and `types` modes the id type is a string (the key), and `FindByCode` works on the
  rows read from the file.
- `types` mode writes no `let` value (DECISIONS 349): a number the code needs at compile time is
  a `const`, and the table entry reads it; a `const` cannot read a `let`.

## Mistakes

| Mistake | Result |
|---|---|
| renaming a key (`draft` to `start`) | `E6001`: the old key was removed, the new one holds its value; retire and add |
| deleting an entry | `E6001`: add `retired` in front of the key instead |
| reusing a code or putting a locked one on another entry | `E6002` (`E3102` in the same table) |
| removing `retired` from an entry | `E6002`: retirement is one-way |
| `@stable` on a plain `table` | `E6003` |
| deleting `canon.lock` lines to make an error go away | no error: and the guarantee is gone; the history of the lock must only gain lines |

Retire with `canon edit` (`{"op": "retire", ...}`) so the lock and the sources change together
(`canon guide recipes`). Delete a lock line only in a reviewed commit that means to forget an id.
