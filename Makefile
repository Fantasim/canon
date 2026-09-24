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

.PHONY: check fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check audit audit-tighten

check: fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; if [ -n "$$out" ]; then echo "gofmt -l: not formatted:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

goldens-vet:
	@for m in $(GOLDEN_MODS); do echo "golden module $$m"; (cd $$m && go vet ./... && go test ./...) || exit 1; done

# Examples whose compiler build cannot run yet: pipeline is illustrative until GEN-01 regenerates
# it (M2, DOCTRINE.md §4); an example that forces `load` fails with build.ErrLoad until M3
# (DECISIONS 196). Add to this list, never delete a MANIFEST, when a new example cannot build.
GOLDEN_BUILD_SKIP := pipeline
# Roots examples/project.canon declares that only a build writes to (examples/_fixtures/README.md);
# resource and client are redirected to the fixtures instead, read-only.
GOLDEN_WRITE_ROOTS := source services sovcommon web parity generated

# Every expected/MANIFEST line names a golden that exists, and every file of that expected/ is
# listed (findings.txt, MANIFEST and go.mod excepted). Then, for every buildable example
# (GOLDEN_BUILD_SKIP aside), rebuild it into a temporary copy of examples/ with every root
# outside the project redirected (examples/_fixtures/README.md) and diff each listed file
# against its golden (IMPLEMENTATION-PLAN.md §7.1).
goldens-check:
	@status=0; for m in $$(find examples -path '*/expected/MANIFEST' | sort); do \
	  d=$$(dirname $$m); \
	  for g in $$(awk 'NF != 2 { print "BAD:" NR; next } { print $$2 }' $$m); do \
	    if [ ! -f "$$d/$$g" ]; then echo "goldens-check: $$m names $$g, which does not exist"; status=1; fi; \
	  done; \
	  for f in $$(cd $$d && find . -type f ! -name findings.txt ! -name MANIFEST ! -name go.mod | sed 's|^\./||' | sort); do \
	    if ! awk '{ print $$2 }' $$m | grep -qx "$$f"; then echo "goldens-check: $$d/$$f is not listed in $$m"; status=1; fi; \
	  done; \
	  ex=$$(basename $$(dirname $$d)); \
	  case " $(GOLDEN_BUILD_SKIP) " in *" $$ex "*) continue ;; esac; \
	  sel="$$ex"; \
	  case "$$ex" in teamboard) sel="teamboard sovcommon..." ;; esac; \
	  tmp=$$(mktemp -d); \
	  cp -r examples/. "$$tmp/proj"; \
	  for r in $(GOLDEN_WRITE_ROOTS); do mkdir -p "$$tmp/out/$$r"; done; \
	  rootargs="--root resource=$$tmp/proj/_fixtures/resource --root client=$$tmp/proj/_fixtures/client"; \
	  for r in $(GOLDEN_WRITE_ROOTS); do rootargs="$$rootargs --root $$r=$$tmp/out/$$r"; done; \
	  if ! go run ./cmd/canon build --project "$$tmp/proj" $$rootargs --target go --target json $$sel > "$$tmp/build.log" 2>&1; then \
	    echo "goldens-check: canon build $$sel failed:"; cat "$$tmp/build.log"; status=1; rm -rf "$$tmp"; continue; \
	  fi; \
	  while read -r display golden; do \
	    case "$$display" in \
	      @*) rest=$${display#@}; root=$${rest%%/*}; sub=$${rest#*/} ;; \
	      *) rest="" ;; \
	    esac; \
	    if [ -n "$$rest" ]; then \
	      case " $(GOLDEN_WRITE_ROOTS) " in \
	        *" $$root "*) src="$$tmp/out/$$root/$$sub" ;; \
	        *) src="$$tmp/proj/$$root/$$sub" ;; \
	      esac; \
	    else \
	      src="$$tmp/proj/$$display"; \
	    fi; \
	    if ! diff -u "$$src" "$$d/$$golden"; then status=1; fi; \
	  done < "$$m"; \
	  rm -rf "$$tmp"; \
	done; exit $$status

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
