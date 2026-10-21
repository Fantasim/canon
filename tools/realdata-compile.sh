#!/usr/bin/env bash
# Release gate (`make realdata-compile`, DECISIONS 394, IMPLEMENTATION-PLAN.md §7.5; real data
# stays out of the repository, DECISIONS 29).
#
# Builds the real Sovereign Canon project with the compiler built from this tree, then compiles
# every generated unit separately at the consumer's flags, each under a memory cap:
#   - C++        every *.gen.cpp: clang++ -O3 -g -std=c++20 -Wall -Wextra -Werror -c
#   - Go         every generated package of the four services: go build, one compile per package
#   - TypeScript every generated *.ts, once per compiler of tools/tsc, with the repository's flags
# A unit that does not compile, goes over MAX_RSS_KB of peak RSS or MAX_SECS seconds fails the
# gate. Prints a table sorted by unit (kind, unit, peak RSS, time) and a PASS/FAIL line.
#
# The project is copied (Resource without its heavy asset trees, Source without its build output,
# each service's go.mod and go.sum) into a scratch tree of the run, with the layout the project's
# relative roots expect, so `canon build` never writes into the real trees. Nothing from the copy
# is kept: the scratch tree is removed on exit. Read-only on $SOVEREIGN.
#
# Usage:  SOVEREIGN=../.. bash tools/realdata-compile.sh
#   SOVEREIGN  the Sovereign root (Resource/, Source/, services/), default ../.. of this repository.
#   RD_JOBS    parallel compiles, default min(nproc/2, 4).
# Exit: 0 PASS, or skipped (no Sovereign tree); 1 FAIL; 2 a needed tool is missing.
set -euo pipefail
export LC_ALL=C

# The gate (DECISIONS 394): a unit above either limit fails.
readonly MAX_RSS_KB=1572864 # 1.5 GB
readonly MAX_SECS=60
# Runaway guard (the unit is a FAIL anyway) and the per-unit memory cap (cgroup, swap off).
readonly KILL_SECS=300
readonly MEM_CAP=4G
readonly MAX_JOBS=4
readonly SERVICES=(monitoring admin website trello)
readonly TS_COMPILERS=(typescript5 typescript-latest)
readonly TS_FLAGS=(--noEmit --strict --noUnusedLocals --target ES2020 --module ES2020 --moduleResolution Bundler --types node)
readonly FAIL_LINES=30 # lines of a failing unit's output shown

self=$(readlink -f "$0")
repo=$(dirname "$(dirname "$self")")
ts_modules="$repo/tools/tsc/node_modules"

# measure <kind> <unit> <command...>: runs the command under the memory cap and the runaway
# timeout, appends "kind unit peak-RSS-KB seconds exit log" to $RD_RESULTS, returns its exit code.
measure() {
  local kind=$1 unit=$2
  shift 2
  local log rss secs rc
  log=$(mktemp "$RD_LOGS/unit.XXXXXX")
  systemd-run --user --scope -q -p MemoryMax="$MEM_CAP" -p MemorySwapMax=0 \
    timeout "$KILL_SECS" /usr/bin/time -f 'RDTIME %M %e %x' "$@" > "$log" 2>&1 || true
  read -r _ rss secs rc < <(grep -a '^RDTIME' "$log" | tail -1 || true) || true
  if [ -z "${rc:-}" ] || grep -aq '^Command terminated by signal' "$log"; then
    rss=${rss:-0} secs=${secs:-0} rc=137 # killed: the cap or a signal
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$kind" "$unit" "$rss" "$secs" "$rc" "$log" >> "$RD_RESULTS"
  return "$rc"
}

# Worker modes, re-entered through xargs and go -toolexec.
case "${1:-}" in
  _cxx)
    measure cxx "$2" clang++ -O3 -g -std=c++20 -Wall -Wextra -Werror -c "-I$(dirname "$2")" "$2" -o /dev/null || true
    exit 0
    ;;
  _ts)
    measure "ts/$2" "$3" node "$ts_modules/$2/bin/tsc" "${TS_FLAGS[@]}" --typeRoots "$ts_modules/@types" "$3" || true
    exit 0
    ;;
  _gotool)
    tool=$2
    shift 2
    if [ "$(basename "$tool")" != compile ]; then exec "$tool" "$@"; fi
    pkg=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = -p ]; then pkg=$a; fi
      prev=$a
    done
    case "$pkg" in
      "$RD_MODPFX" | "$RD_MODPFX"/*) ;;
      *) exec "$tool" "$@" ;;
    esac
    rc=0
    measure go "$pkg" "$tool" "$@" || rc=$?
    exit "$rc"
    ;;
  "") ;;
  *)
    echo "realdata-compile: unknown mode $1" >&2
    exit 2
    ;;
esac

sovereign=${SOVEREIGN:-$repo/../..}
if ! sovereign=$(cd "$sovereign" 2> /dev/null && pwd); then
  echo "realdata-compile: SKIPPED, no Sovereign tree at ${SOVEREIGN:-$repo/../..} (set SOVEREIGN=<root>)"
  exit 0
fi
missing=()
for need in Resource/Config/project.canon Source; do
  if [ ! -e "$sovereign/$need" ]; then missing+=("$need"); fi
done
for svc in "${SERVICES[@]}"; do
  if [ ! -f "$sovereign/services/$svc/go.mod" ]; then missing+=("services/$svc/go.mod"); fi
done
if [ "${#missing[@]}" -gt 0 ]; then
  echo "realdata-compile: SKIPPED, $sovereign lacks: ${missing[*]}"
  exit 0
fi

for tool in clang++ go node systemd-run rsync /usr/bin/time; do
  if ! command -v "$tool" > /dev/null; then
    echo "realdata-compile: $tool not found on PATH" >&2
    exit 2
  fi
done
for c in "${TS_COMPILERS[@]}"; do
  if [ ! -f "$ts_modules/$c/bin/tsc" ]; then
    echo "realdata-compile: $c not installed (npm ci --prefix tools/tsc)" >&2
    exit 2
  fi
done

jobs=${RD_JOBS:-$(($(nproc) / 2))}
if [ "$jobs" -gt "$MAX_JOBS" ]; then jobs=$MAX_JOBS; fi
if [ "$jobs" -lt 1 ]; then jobs=1; fi

work=$(mktemp -d "${TMPDIR:-/var/tmp}/canon-realdata.XXXXXX")
trap 'rm -rf "$work"' EXIT
proj="$work/proj"
export RD_RESULTS="$work/results.tsv" RD_LOGS="$work/logs"
mkdir -p "$proj/services" "$RD_LOGS"
: > "$RD_RESULTS"

phase() {
  echo "realdata-compile: $1 (+$(($(date +%s) - start)) s)"
}

die() {
  echo "realdata-compile: FAIL ($1)"
  exit 1
}

start=$(date +%s)
echo "realdata-compile: Sovereign $sovereign, $jobs jobs, scratch tree in $work"
(cd "$repo" && GOTOOLCHAIN=local go build -o "$work/canon" ./cmd/canon) || die "building canon"

phase "copying the project"
rsync -a --exclude=/.git --exclude=/Model --exclude=/World --exclude=/UI --exclude=/Effects \
  --exclude=/Audio "$sovereign/Resource/" "$proj/Resource/" || die "copying Resource"
rsync -a --exclude=/.git --exclude=/build --exclude=/External "$sovereign/Source/" "$proj/Source/" ||
  die "copying Source"
for svc in "${SERVICES[@]}"; do
  mkdir -p "$proj/services/$svc"
  cp "$sovereign/services/$svc/go.mod" "$proj/services/$svc/"
  if [ -f "$sovereign/services/$svc/go.sum" ]; then cp "$sovereign/services/$svc/go.sum" "$proj/services/$svc/"; fi
done

phase "canon build"
cd "$proj/Resource/Config"
"$work/canon" build --max-warnings 100000 > "$work/build.log" 2>&1 || {
  tail -n "$FAIL_LINES" "$work/build.log"
  die "canon build"
}
grep -a 'errors,' "$work/build.log" | tail -n 1 || true
for svc in "${SERVICES[@]}"; do
  "$work/canon" build --max-warnings 100000 --only-root "$svc" > "$work/build-$svc.log" 2>&1 || {
    tail -n "$FAIL_LINES" "$work/build-$svc.log"
    die "canon build --only-root $svc"
  }
done
cd "$proj"

phase "C++"
find Source -name '*.gen.cpp' -print0 | sort -z | xargs -0 -r -P "$jobs" -n 1 "$self" _cxx

phase "TypeScript"
for c in "${TS_COMPILERS[@]}"; do
  find services -name '*.ts' -not -path '*/node_modules/*' -print0 | sort -z |
    xargs -0 -r -P "$jobs" -I '{}' "$self" _ts "$c" '{}'
done

# Go: one compile per generated package, timed by -toolexec; the standard library and the
# dependencies compile unmeasured. Offline: the module cache must hold the dependencies.
export GOTOOLCHAIN=local GOFLAGS=-mod=mod GOPROXY=off GOWORK=off GOCACHE="$work/gocache"
for svc in "${SERVICES[@]}"; do
  phase "Go $svc"
  RD_MODPFX=$(awk '$1 == "module" { print $2; exit }' "services/$svc/go.mod")
  export RD_MODPFX
  if ! (cd "services/$svc" && go build -p "$jobs" -toolexec "$self _gotool" ./... > "$work/go-$svc.log" 2>&1); then
    if ! grep -aq . "$work/go-$svc.log"; then echo "go build failed" > "$work/go-$svc.log"; fi
    printf 'go\tgo build services/%s\t0\t0\t1\t%s\n' "$svc" "$work/go-$svc.log" >> "$RD_RESULTS"
  fi
done

# The table, sorted by kind and unit; a row fails on a non-zero exit, on peak RSS or on time.
failed="$work/failed.tsv"
: > "$failed"
sort -t $'\t' -k1,1 -k2,2 "$RD_RESULTS" | awk -F '\t' -v maxrss="$MAX_RSS_KB" -v maxsecs="$MAX_SECS" -v failed="$failed" '
  { why = ""
    if ($5 != 0) why = "exit " $5
    else if ($3 > maxrss) why = sprintf("over %d MB", maxrss / 1024)
    else if ($4 > maxsecs) why = "over " maxsecs " s"
    kind[NR] = $1; unit[NR] = $2; rss[NR] = $3; secs[NR] = $4; res[NR] = why == "" ? "ok" : "FAIL " why
    if (why != "") print $2 "\t" $6 > failed
    w = length($2); if (w > uw) uw = w }
  END { printf "%-8s %-*s %10s %8s  %s\n", "kind", uw, "unit", "peak RSS", "time", "result"
        for (i = 1; i <= NR; i++)
          printf "%-8s %-*s %7.0f MB %6.2f s  %s\n", kind[i], uw, unit[i], rss[i] / 1024, secs[i], res[i] }'

units=$(wc -l < "$RD_RESULTS")
nfail=$(wc -l < "$failed")
elapsed=$(($(date +%s) - start))
while IFS=$'\t' read -r unit log; do
  echo "--- $unit"
  { grep -av '^RDTIME' "$log" | head -n "$FAIL_LINES" | sed "s|$work/||g"; } || true
done < "$failed"
if [ "$units" -eq 0 ]; then die "no generated unit found"; fi
if [ "$nfail" -gt 0 ]; then
  echo "realdata-compile: FAIL ($nfail of $units units, ${elapsed}s)"
  exit 1
fi
echo "realdata-compile: PASS ($units units, limits ${MAX_RSS_KB} KB and ${MAX_SECS} s, ${elapsed}s)"
