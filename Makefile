# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

TOOLS_MOD_DIR := ./internal/tools
TOOLS = $(CURDIR)/.tools

ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' ! -path './LICENSES/*' -exec dirname {} \; | sort)

# Build the list of include directories to compile the bpf program
BPF_INCLUDE += -I${REPODIR}/internal/include/libbpf
BPF_INCLUDE += -I${REPODIR}/internal/include

# Go default variables
GOCMD?= go
GOOS=linux
CGO_ENABLED=0

.DEFAULT_GOAL := precommit

.PHONY: precommit
precommit: license-header-check dependabot-generate go-mod-tidy golangci-lint-fix

# Tools
$(TOOLS):
	@mkdir -p $@
$(TOOLS)/%: | $(TOOLS)
	cd $(TOOLS_MOD_DIR) && \
	$(GOCMD) build  -buildvcs=false -o $@ $(PACKAGE)

MULTIMOD = $(TOOLS)/multimod
$(TOOLS)/multimod: PACKAGE=go.opentelemetry.io/build-tools/multimod

GOLICENSES = $(TOOLS)/go-licenses
$(TOOLS)/go-licenses: PACKAGE=github.com/google/go-licenses

DBOTCONF = $(TOOLS)/dbotconf
$(TOOLS)/dbotconf: PACKAGE=go.opentelemetry.io/build-tools/dbotconf

IMG_NAME ?= otel-go-instrumentation
IMG_NAME_BASE = $(IMG_NAME)-base

GOLANGCI_LINT = $(TOOLS)/golangci-lint
$(TOOLS)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint

OFFSETGEN = $(TOOLS)/offsetgen
$(TOOLS)/offsetgen: PACKAGE=go.opentelemetry.io/auto/$(TOOLS_MOD_DIR)/inspect/cmd/offsetgen

.PHONY: tools
tools: $(GOLICENSES) $(MULTIMOD) $(GOLANGCI_LINT) $(DBOTCONF) $(OFFSETGEN)

ALL_GO_MODS := $(shell find . -type f -name 'go.mod' ! -path '$(TOOLS_MOD_DIR)/*' ! -path './LICENSES/*' | sort)
GO_MODS_TO_TEST := $(ALL_GO_MODS:%=test/%)

.PHONY: test
test: generate $(GO_MODS_TO_TEST)
test/%: GO_MOD=$*
test/%:
	cd $(shell dirname $(GO_MOD)) && $(GOCMD) test -v ./...

.PHONY: generate
generate: export CFLAGS := $(BPF_INCLUDE)
generate: go-mod-tidy
generate:
	$(GOCMD) generate ./...

.PHONY: docker-generate
docker-generate: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd ../app && make generate"

.PHONY: docker-test
docker-test: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd ../app && make test"

.PHONY: docker-precommit
docker-precommit: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd ../app && make precommit"

.PHONY: go-mod-tidy
go-mod-tidy: $(ALL_GO_MOD_DIRS:%=go-mod-tidy/%)
go-mod-tidy/%: DIR=$*
go-mod-tidy/%:
	@cd $(DIR) && $(GOCMD) mod tidy -compat=1.20

.PHONY: golangci-lint golangci-lint-fix
golangci-lint-fix: ARGS=--fix
golangci-lint-fix: golangci-lint
golangci-lint: generate $(ALL_GO_MOD_DIRS:%=golangci-lint/%)
golangci-lint/%: DIR=$*
golangci-lint/%: | $(GOLANGCI_LINT)
	@echo 'golangci-lint $(if $(ARGS),$(ARGS) ,)$(DIR)' \
		&& cd $(DIR) \
		&& $(GOLANGCI_LINT) run --allow-serial-runners --timeout=2m0s $(ARGS)

.PHONY: build
build: generate
	$(GOCMD) build -o otel-go-instrumentation cli/main.go

.PHONY: docker-build
docker-build:
	docker buildx build -t $(IMG_NAME) .

.PHONY: docker-build-base
docker-build-base:
	docker buildx build -t $(IMG_NAME_BASE) --target base .

OFFSETS_OUTPUT_FILE="$(REPODIR)/internal/pkg/inject/offset_results.json"
.PHONY: offsets
offsets: | $(OFFSETGEN)
	$(OFFSETGEN) -output=$(OFFSETS_OUTPUT_FILE) -cache=$(OFFSETS_OUTPUT_FILE)

.PHONY: docker-offsets
docker-offsets:
	docker run --rm -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock -v $(shell pwd):/app golang:1.22 /bin/sh -c "cd ../app && make offsets"

.PHONY: update-licenses
update-licenses: generate $(GOLICENSES)
	rm -rf LICENSES
	$(GOLICENSES) save ./cli/ --save_path LICENSES
	cp -R ./internal/include/libbpf ./LICENSES

.PHONY: verify-licenses
verify-licenses: generate $(GOLICENSES)
	$(GOLICENSES) save ./cli --save_path temp
	cp -R ./internal/include/libbpf ./temp; \
    if diff temp LICENSES > /dev/null; then \
      echo "Passed"; \
      rm -rf temp; \
    else \
      echo "LICENSES directory must be updated. Run make update-licenses"; \
      rm -rf temp; \
      exit 1; \
    fi; \

DEPENDABOT_CONFIG = .github/dependabot.yml
.PHONY: dependabot-check
dependabot-check: | $(DBOTCONF)
	@$(DBOTCONF) --ignore "/LICENSES" verify $(DEPENDABOT_CONFIG) || ( echo "(run: make dependabot-generate)"; exit 1 )

.PHONY: dependabot-generate
dependabot-generate: | $(DBOTCONF)
	@$(DBOTCONF) --ignore "/LICENSES" generate > $(DEPENDABOT_CONFIG)

.PHONY: license-header-check
license-header-check:
	@licRes=$$(for f in $$(find . -type f \( -iname '*.go' -o -iname '*.sh' \) ! -path '**/third_party/*' ! -path './.git/*' ! -path './LICENSES/*' ) ; do \
	           awk '/Copyright The OpenTelemetry Authors|generated|GENERATED/ && NR<=3 { found=1; next } END { if (!found) print FILENAME }' $$f; \
	   done); \
	   if [ -n "$${licRes}" ]; then \
	           echo "license header checking failed:"; echo "$${licRes}"; \
	           exit 1; \
	   fi

.PHONY: fixture-nethttp fixture-gin fixture-databasesql fixture-nethttp-custom fixture-otelglobal fixture-kafka-go
fixture-nethttp-custom: fixtures/nethttp_custom
fixture-nethttp: fixtures/nethttp
fixture-gin: fixtures/gin
fixture-databasesql: fixtures/databasesql
fixture-grpc: fixtures/grpc
fixture-otelglobal: fixtures/otelglobal
fixture-kafka-go: fixtures/kafka-go
fixtures/%: LIBRARY=$*
fixtures/%:
	$(MAKE) docker-build
	cd internal/test/e2e/$(LIBRARY) && docker build -t sample-app .
	kind create cluster
	kind load docker-image otel-go-instrumentation sample-app
	helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
	if [ ! -d "opentelemetry-helm-charts" ]; then \
		git clone https://github.com/open-telemetry/opentelemetry-helm-charts.git; \
	fi
	if [ -f ./internal/test/e2e/$(LIBRARY)/collector-helm-values.yml ]; then \
		helm install test -f ./internal/test/e2e/$(LIBRARY)/collector-helm-values.yml opentelemetry-helm-charts/charts/opentelemetry-collector; \
	else \
		helm install test -f .github/workflows/e2e/k8s/collector-helm-values.yml opentelemetry-helm-charts/charts/opentelemetry-collector; \
	fi
	while : ; do \
		kubectl get pod/test-opentelemetry-collector-0 && break; \
		sleep 5; \
	done
	kubectl wait --for=condition=Ready --timeout=60s pod/test-opentelemetry-collector-0
	kubectl -n default create -f .github/workflows/e2e/k8s/sample-job.yml
	if kubectl wait --for=condition=Complete --timeout=60s job/sample-job; then \
		kubectl cp -c filecp default/test-opentelemetry-collector-0:tmp/trace.json ./internal/test/e2e/$(LIBRARY)/traces-orig.json; \
		rm -f ./internal/test/e2e/$(LIBRARY)/traces.json; \
		bats ./internal/test/e2e/$(LIBRARY)/verify.bats; \
	else \
		kubectl logs -l app=sample -c auto-instrumentation; \
	fi
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
