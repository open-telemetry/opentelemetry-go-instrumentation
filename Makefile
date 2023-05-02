# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

TOOLS_MOD_DIR := ./internal/tools
TOOLS = $(CURDIR)/.tools

ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' ! -path './LICENSES/*' -exec dirname {} \; | sort)
OTEL_GO_MOD_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(ALL_GO_MOD_DIRS))

# Build the list of include directories to compile the bpf program
BPF_INCLUDE += -I${REPODIR}/include/libbpf
BPF_INCLUDE += -I${REPODIR}/include

.DEFAULT_GOAL := precommit

.PHONY: precommit
precommit: license-header-check go-mod-tidy golangci-lint-fix

# Tools
$(TOOLS):
	@mkdir -p $@
$(TOOLS)/%: | $(TOOLS)
	cd $(TOOLS_MOD_DIR) && \
	go build -o $@ $(PACKAGE)

MULTIMOD = $(TOOLS)/multimod
$(TOOLS)/multimod: PACKAGE=go.opentelemetry.io/build-tools/multimod

GOLICENSES = $(TOOLS)/go-licenses
$(TOOLS)/go-licenses: PACKAGE=github.com/google/go-licenses

IMG_NAME ?= otel-go-instrumentation

GOLANGCI_LINT = $(TOOLS)/golangci-lint
$(TOOLS)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: tools
tools: $(GOLICENSES) $(MULTIMOD) $(GOLANGCI_LINT)

ALL_GO_MODS := $(shell find . -type f -name 'go.mod' ! -path '$(TOOLS_MOD_DIR)/*' ! -path './LICENSES/*' | sort)
GO_MODS_TO_TEST := $(ALL_GO_MODS:%=test/%)

.PHONY: test
test: $(GO_MODS_TO_TEST)
test/%: GO_MOD=$*
test/%:
	cd $(shell dirname $(GO_MOD)) && go test -v ./...

.PHONY: generate
generate: export CFLAGS := $(BPF_INCLUDE)
generate: go-mod-tidy
generate:
	go generate ./...

.PHONY: go-mod-tidy
go-mod-tidy: $(ALL_GO_MOD_DIRS:%=go-mod-tidy/%)
go-mod-tidy/%: DIR=$*
go-mod-tidy/%:
	@cd $(DIR) && go mod tidy -compat=1.20

.PHONY: golangci-lint golangci-lint-fix
golangci-lint-fix: ARGS=--fix
golangci-lint-fix: golangci-lint
golangci-lint: generate $(OTEL_GO_MOD_DIRS:%=golangci-lint/%)
golangci-lint/%: DIR=$*
golangci-lint/%: | $(GOLANGCI_LINT)
	@echo 'golangci-lint $(if $(ARGS),$(ARGS) ,)$(DIR)' \
		&& cd $(DIR) \
		&& $(GOLANGCI_LINT) run --allow-serial-runners $(ARGS)

.PHONY: build
build: generate
	GOOS=linux go build -o otel-go-instrumentation cli/main.go

.PHONY: docker-build
docker-build:
	docker buildx build -t $(IMG_NAME) .

.PHONY: offsets
offsets:
	cd offsets-tracker; OFFSETS_OUTPUT_FILE="../pkg/inject/offset_results.json" go run main.go

.PHONY: docker-offsets
docker-offsets:
	docker run --rm -v $(shell pwd):/app golang:1.20 /bin/sh -c "cd ../app && make offsets"

.PHONY: update-licenses
update-licenses: generate $(GOLICENSES)
	rm -rf LICENSES
	$(GOLICENSES) save ./cli/ --save_path LICENSES
	cp -R ./include/libbpf ./LICENSES

.PHONY: verify-licenses
verify-licenses: generate $(GOLICENSES)
	$(GOLICENSES) save ./cli --save_path temp
	cp -R ./include/libbpf ./temp; \
    if diff temp LICENSES > /dev/null; then \
      echo "Passed"; \
      rm -rf temp; \
    else \
      echo "LICENSES directory must be updated. Run make update-licenses"; \
      rm -rf temp; \
      exit 1; \
    fi; \

.PHONY: license-header-check
license-header-check:
	@licRes=$$(for f in $$(find . -type f \( -iname '*.go' -o -iname '*.sh' \) ! -path '**/third_party/*' ! -path './.git/*' ! -path './LICENSES/*' ) ; do \
	           awk '/Copyright The OpenTelemetry Authors|generated|GENERATED/ && NR<=3 { found=1; next } END { if (!found) print FILENAME }' $$f; \
	   done); \
	   if [ -n "$${licRes}" ]; then \
	           echo "license header checking failed:"; echo "$${licRes}"; \
	           exit 1; \
	   fi

.PHONY: fixture-nethttp fixture-gorillamux fixture-gin
fixture-nethttp: fixtures/nethttp
fixture-gorillamux: fixtures/gorillamux
fixture-gin: fixtures/gin
fixtures/%: LIBRARY=$*
fixtures/%:
	$(MAKE) docker-build
	cd test/e2e/$(LIBRARY) && docker build -t sample-app .
	kind create cluster
	kind load docker-image otel-go-instrumentation sample-app
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	if [ ! -d "opentelemetry-helm-charts" ]; then \
		git clone https://github.com/open-telemetry/opentelemetry-helm-charts.git; \
	fi
	helm install test -f .github/workflows/e2e/k8s/collector-helm-values.yml opentelemetry-helm-charts/charts/opentelemetry-collector
	kubectl wait --for=condition=Ready --timeout=60s pod/test-opentelemetry-collector-0
	kubectl -n default create -f .github/workflows/e2e/k8s/sample-job.yml
	kubectl wait --for=condition=Complete --timeout=60s job/sample-job
	kubectl cp -c filecp default/test-opentelemetry-collector-0:tmp/trace.json ./test/e2e/$(LIBRARY)/traces.json.tmp
	jq 'del(.resourceSpans[].scopeSpans[].spans[].endTimeUnixNano, .resourceSpans[].scopeSpans[].spans[].startTimeUnixNano) | .resourceSpans[].scopeSpans[].spans[].spanId|= (if . != "" then "xxxxx" else . end) | .resourceSpans[].scopeSpans[].spans[].traceId|= (if . != "" then "xxxxx" else . end) | .resourceSpans[].scopeSpans|=sort_by(.scope.name)' ./test/e2e/$(LIBRARY)/traces.json.tmp | jq --sort-keys . > ./test/e2e/$(LIBRARY)/traces.json
	rm ./test/e2e/$(LIBRARY)/traces.json.tmp
	kind delete cluster

.PHONY: prerelease
prerelease: | $(MULTIMOD)
	@[ "${MODSET}" ] || ( echo ">> env var MODSET is not set"; exit 1 )
	$(MULTIMOD) verify && $(MULTIMOD) prerelease -m ${MODSET}

COMMIT ?= "HEAD"
.PHONY: add-tags
add-tags: | $(MULTIMOD)
	@[ "${MODSET}" ] || ( echo ">> env var MODSET is not set"; exit 1 )
	$(MULTIMOD) verify && $(MULTIMOD) tag -m ${MODSET} -c ${COMMIT}

.PHONY: check-clean-work-tree
check-clean-work-tree:
	if [ -n "$$(git status --porcelain)" ]; then \
		git status; \
		git --no-pager diff; \
		echo 'Working tree is not clean, did you forget to run "make precommit", "make generate" or "make offsets"?'; \
		exit 1; \
	fi
