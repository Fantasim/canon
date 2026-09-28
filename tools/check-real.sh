#!/usr/bin/env bash
# Real-data job (`make check-real`, IMPLEMENTATION-PLAN.md §6 M3 item 7, §7.3; DECISIONS 29).
#
# Runs `canon check` on every example package that reads the `resource` root (found by grepping
# `@resource` in examples/**/*.canon), plus balance.parity, against the real Resource tree kept
# in the git-ignored testdata-real/. `client` stays redirected to examples/_fixtures/client (the
# real tree has no Client/Text, meta/decisions/log-2026-09-28.md decision 9); every written root
# points at a scratch directory so the job never writes into a real tree or this repository.
#
# Non-gating: never part of `make check`. Exits 0 even when `canon check` finds errors or
# warnings; a non-zero exit means the job itself could not run (no testdata-real/, a build
# failure, or `canon check` failing for a reason other than findings).
set -euo pipefail
export LC_ALL=C

cd "$(dirname "$0")/.."

real_dir=testdata-real
resource_root="$real_dir/Resource"
commit_file="$real_dir/RESOURCE_COMMIT"

if [ ! -d "$resource_root" ] || [ ! -f "$commit_file" ]; then
  echo "check-real: $real_dir/ (Resource/, RESOURCE_COMMIT) not found; see DECISIONS 29" >&2
  exit 2
fi

out_dir="$real_dir/realdata"
mkdir -p "$out_dir"

tmpbase=${TMPDIR:-/var/tmp}
bin_dir=$(mktemp -d "$tmpbase/canon-check-real-bin.XXXXXX")
scratch=$(mktemp -d "$tmpbase/canon-check-real-scratch.XXXXXX")
tmpfile=$(mktemp "$tmpbase/canon-check-real-out.XXXXXX")
trap 'rm -rf "$bin_dir" "$scratch" "$tmpfile"' EXIT

go build -o "$bin_dir/canon" ./cmd/canon

# Every example package under examples/**/*.canon that reads `@resource`, plus balance.parity
# (item 7's own name), sorted.
packages=(
  balance.parity
  features.legacycpp
  features.text
  game.items
  resource.adventurequest
  resource.events
  resource.farm
  resource.heistia
  resource.rules
  resource.vocab
)

roots=(
  --root "resource=$(pwd)/$resource_root"
  --root "client=$(pwd)/examples/_fixtures/client"
)
for name in source services sovcommon web parity generated; do
  roots+=(--root "$name=$scratch/$name")
done

resource_commit=$(tr -d '\n' < "$commit_file")
canon_rev=$(git rev-parse HEAD)

printf '# real-data findings: canon %s, Resource %s\n\n' "$canon_rev" "$resource_commit" > "$tmpfile"

fail=0
for pkg in "${packages[@]}"; do
  printf '=== %s ===\n' "$pkg" >> "$tmpfile"
  set +e
  pkg_out=$("$bin_dir/canon" check --project examples "${roots[@]}" --format text "$pkg" 2>&1)
  code=$?
  set -e
  printf '%s\n\n' "$pkg_out" >> "$tmpfile"
  # exit 0 (clean), 1 (errors found) and 4 (max-warnings) are findings, not a tool failure
  # (internal/cli/constants.go); 2, 3 and 130 mean canon check itself could not run.
  case "$code" in
    0 | 1 | 4) ;;
    *) fail=1 ;;
  esac
done

sed -E 's/\(([0-9]+ ms|[0-9]+\.[0-9] s)\)/(…)/g' "$tmpfile" > "$out_dir/findings.txt"

exit "$fail"
