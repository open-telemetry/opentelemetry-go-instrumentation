# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' ! -path './LICENSES/*' -exec dirname {} \; | sort)

# Build the list of include directories to compile the bpf program
BPF_INCLUDE += -I${REPODIR}/include/libbpf
BPF_INCLUDE+= -I${REPODIR}/include

.DEFAULT_GOAL := precommit

.PHONY: precommit
precommit: license-header-check go-mod-tidy

# Tools
TOOLS_MOD_DIR := ./internal/tools
TOOLS = $(CURDIR)/.tools

$(TOOLS):
	@mkdir -p $@
$(TOOLS)/%: | $(TOOLS)
	cd $(TOOLS_MOD_DIR) && \
	go build -o $@ $(PACKAGE)

GOLICENSES = $(TOOLS)/go-licenses
$(TOOLS)/go-licenses: PACKAGE=github.com/google/go-licenses

.PHONY: tools
tools: $(GOLICENSES)

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
	go generate ./...

.PHONY: go-mod-tidy
go-mod-tidy: $(ALL_GO_MOD_DIRS:%=go-mod-tidy/%)
go-mod-tidy/%: DIR=$*
go-mod-tidy/%:
	@cd $(DIR) && go mod tidy -compat=1.20

.PHONY: build
build: generate
	GOOS=linux GOARCH=amd64 go build -o otel-go-instrumentation cli/main.go

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

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
	IMG=otel-go-instrumentation $(MAKE) docker-build
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

.PHONY: check-clean-work-tree
check-clean-work-tree:
	@if ! git diff --quiet; then \
	  echo; \
	  echo 'Working tree is not clean, did you forget to run "make precommit"?'; \
	  echo; \
	  git status; \
	  exit 1; \
	fi
