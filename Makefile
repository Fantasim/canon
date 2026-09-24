# The gate every change passes (DECISIONS.md, decision 25). `make check` runs, in order:
# formatting, vet, tests, the generated goldens, the diagnostics registry, then the code audit
# (tools/audit) on itself and on the repository (IMPLEMENTATION-PLAN.md §12.2).
# Tip: GOTOOLCHAIN=local make check never downloads a Go toolchain.
# examples/go.mod holds no package: it only keeps the goldens out of the compiler module, so
# nothing vets it; each golden Go module has its own go.mod and is vetted by goldens-vet.

AUDIT_DIR   := tools/audit
AUDIT       := cd $(AUDIT_DIR) && go run .
# Every Go module holding generated goldens: vetted and tested in place, never formatted or
# audited (the compiler writes them; goldens-check diffs them).
GOLDEN_MODS := $(patsubst %/go.mod,%,$(shell find examples -path '*/expected/*' -name go.mod 2>/dev/null))
# Hand-written Go files: everything but goldens, generated files, fixtures and testdata.
GO_FILES     = $(shell find . -name '*.go' -not -path './examples/*/expected/*' \
                 -not -path './examples/_fixtures/*' -not -path '*/testdata/*' \
                 -not -path './.claude/*' -not -name '*.gen.go')

.PHONY: check fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check audit audit-tighten scope

check: fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; if [ -n "$$out" ]; then echo "gofmt -l: not formatted:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

# A golden module with a smoke test (internal/testkit/golden/testdata/smoke/<example>/, DECISIONS
# 201) is copied to a temporary directory with the smoke test added, so expected/ keeps holding
# only compiler output; one without is vetted and tested in place. Assumes one golden Go module
# per example: the smoke directory is named after the example two levels above expected/'s go.mod.
goldens-vet:
	@for m in $(GOLDEN_MODS); do \
	  echo "golden module $$m"; \
	  smoke="internal/testkit/golden/testdata/smoke/$$(basename $$(dirname $$(dirname $$m)))"; \
	  if [ -d "$$smoke" ]; then \
	    tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	    cp -r "$$m/." "$$tmp" && mkdir -p "$$tmp/smoke" && cp -r "$$smoke/." "$$tmp/smoke" && \
	      (cd "$$tmp" && go vet ./... && go test ./...); \
	    status=$$?; rm -rf "$$tmp"; trap - EXIT; \
	    [ $$status -eq 0 ] || exit $$status; \
	  else \
	    (cd $$m && go vet ./... && go test ./...) || exit 1; \
	  fi; \
	done

# Every expected/MANIFEST line names a golden that exists, and every file of that expected/ is
# listed (findings.txt, MANIFEST and go.mod excepted). internal/testkit/golden's TestExamples
# then rebuilds every buildable example (its own skip list, currently pipeline, aside) into a
# temporary copy of examples/ with every outside root redirected and diffs it, failing also when
# the build wrote an output the MANIFEST does not list (IMPLEMENTATION-PLAN.md §7.1, DECISIONS
# 201).
goldens-check:
	@status=0; for m in $$(find examples -path '*/expected/MANIFEST' | sort); do \
	  d=$$(dirname $$m); \
	  for g in $$(awk 'NF != 2 { print "BAD:" NR; next } { print $$2 }' $$m); do \
	    if [ ! -f "$$d/$$g" ]; then echo "goldens-check: $$m names $$g, which does not exist"; status=1; fi; \
	  done; \
	  for f in $$(cd $$d && find . -type f ! -name findings.txt ! -name MANIFEST ! -name go.mod | sed 's|^\./||' | sort); do \
	    if ! awk '{ print $$2 }' $$m | grep -qx "$$f"; then echo "goldens-check: $$d/$$f is not listed in $$m"; status=1; fi; \
	  done; \
	done; \
	go test ./internal/testkit/golden/... -run TestExamples -v || status=1; \
	exit $$status

# internal/diag/codes.go and its generated test table equal what diaggen generates from
# spec/ERRORS.md into a temporary directory, and the runtime helper texts under internal/gen
# use only the pairs of ERRORS.md §1.6 (IMPLEMENTATION-PLAN.md §12.2 step 5).
diag-check:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
	  go run ./internal/diag/cmd/diaggen -out "$$tmp" -runtime internal/gen && \
	  diff -u internal/diag/codes.go "$$tmp/codes.go" && \
	  diff -u internal/diag/codes_test.go "$$tmp/codes_test.go"

# The audit tool gates itself first: its own vet, tests and baseline (tools/audit/.sovaudit).
audit-self:
	cd $(AUDIT_DIR) && go vet ./... && go test ./...
	$(AUDIT) check --repo . --quiet

audit-check:
	$(AUDIT) check --repo ../.. --quiet

# The census: every rule with its count, zeros included, and the first findings of each.
audit:
	$(AUDIT) audit --repo ../..

# After a cleanup: lower .sovaudit/baseline.tsv to what is left (it never grows).
audit-tighten:
	$(AUDIT) baseline --tighten --repo ../..

# DECISIONS 200: the generated-program suites at nightly size, under the memory cap; a crash is
# reported with its seed and the suite goes on; -progen.keep writes the shrunk counterexamples.
PROGEN_N    ?= 5000
PROGEN_SEED ?= 1
.PHONY: progen-nightly
progen-nightly:
	systemd-run --user --scope -q -p MemoryMax=3G env GOTOOLCHAIN=local go test -count=1 -timeout 0 \
	  -run 'TestMutations|TestGrammar|TestCorruption' ./internal/testkit/progen \
	  -progen.n $(PROGEN_N) -progen.seed $(PROGEN_SEED) -progen.keep

# Project-size report: git-tracked code lines, blanks and comments excluded (tools/scope.sh).
scope:
	@bash tools/scope.sh
