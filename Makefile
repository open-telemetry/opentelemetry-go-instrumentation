# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Used by bpf2go to generate make compatible depinfo files.
export BPF2GO_MAKEBASE := $(REPODIR)

TOOLS_MOD_DIR := ./internal/tools
TOOLS = $(CURDIR)/.tools

ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' ! -path './LICENSES/*' -exec dirname {} \; | sort)
ALL_GO_MODS := $(shell find . -type f -name 'go.mod' ! -path '$(TOOLS_MOD_DIR)/*' ! -path './LICENSES/*' | sort)

EXAMPLE_MODS := $(filter ./examples/%,$(ALL_GO_MODS))

# BPF compile time dependencies.
BPF2GO_CFLAGS += -I${REPODIR}/internal/include/libbpf
BPF2GO_CFLAGS += -I${REPODIR}/internal/include
export BPF2GO_CFLAGS

# Go default variables
GOCMD?= go
CGO_ENABLED?=0

# User to run as in docker images.
DOCKER_USER=$(shell id -u):$(shell id -g)
DEPENDENCIES_DOCKERFILE=./dependencies.Dockerfile

.DEFAULT_GOAL := precommit

.PHONY: precommit
precommit: license-header-check golangci-lint-fix test codespell

# Tools
$(TOOLS):
	@mkdir -p $@
$(TOOLS)/%: | $(TOOLS)
	cd $(TOOLS_MOD_DIR) && \
	$(GOCMD) build  -buildvcs=false -o $@ $(PACKAGE)

MULTIMOD = $(TOOLS)/multimod
$(TOOLS)/multimod: PACKAGE=go.opentelemetry.io/build-tools/multimod

GOLICENSES = $(TOOLS)/go-licenses
$(TOOLS)/go-licenses: PACKAGE=github.com/google/go-licenses/v2

DBOTCONF = $(TOOLS)/dbotconf
$(TOOLS)/dbotconf: PACKAGE=go.opentelemetry.io/build-tools/dbotconf

IMG_NAME ?= otel-go-instrumentation
IMG_NAME_BASE = $(IMG_NAME)-base

GOLANGCI_LINT = $(TOOLS)/golangci-lint
$(TOOLS)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint

OFFSETGEN = $(TOOLS)/offsetgen
$(TOOLS)/offsetgen: PACKAGE=go.opentelemetry.io/auto/$(TOOLS_MOD_DIR)/inspect/cmd/offsetgen

SYNCLIBBPF = $(TOOLS)/synclibbpf
$(TOOLS)/synclibbpf: PACKAGE=go.opentelemetry.io/auto/$(TOOLS_MOD_DIR)/synclibbpf

CROSSLINK = $(TOOLS)/crosslink
$(TOOLS)/crosslink: PACKAGE=go.opentelemetry.io/build-tools/crosslink

.PHONY: tools
tools: $(GOLICENSES) $(MULTIMOD) $(GOLANGCI_LINT) $(DBOTCONF) $(OFFSETGEN) $(SYNCLIBBPF) $(CROSSLINK)

TEST_TARGETS := test-verbose test-ebpf test-race
.PHONY: $(TEST_TARGETS) test
test-ebpf: ARGS = -tags=ebpf_test -run ^TestEBPF # These need to be run with sudo.
test-verbose: ARGS = -v
test-race: ARGS = -race
$(TEST_TARGETS): test
test: go-mod-tidy generate $(ALL_GO_MODS:%=test/%)
test/%/go.mod:
	@cd $* && $(GOCMD) test $(ARGS) ./...

PROBE_ROOT = internal/pkg/instrumentation/bpf/
PROBE_GEN_GO := $(shell find $(PROBE_ROOT) -type f -name 'bpf_*_bpfe[lb].go')
PROBE_GEN_OBJ := $(PROBE_GEN_GO:.go=.o)
PROBE_GEN_ALL := $(PROBE_GEN_GO) $(PROBE_GEN_OBJ)

# Include all depinfo files to ensure we only re-generate when needed.
-include $(shell find $(PROBE_ROOT) -type f -name 'bpf_*_bpfel.go.d')

.PHONY: generate generate/all
generate: $(PROBE_GEN_ALL)

$(PROBE_GEN_ALL):
	$(GOCMD) generate ./$(dir $@)...

generate/all:
	$(GOCMD) generate ./...

.PHONY: docker-generate
docker-generate: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd /app && make generate"

.PHONY: docker-test
docker-test: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd /app && make test"

.PHONY: docker-precommit
docker-precommit: docker-build-base
	docker run --rm -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd /app && make precommit"

null  :=
space := $(null) #
comma := ,

.PHONY: crosslink
crosslink: $(CROSSLINK)
	@$(CROSSLINK) --root=$(REPODIR) --skip=$(subst $(space),$(comma),$(strip $(EXAMPLE_MODS:./%=%))) --prune

.PHONY: go-mod-tidy
go-mod-tidy: $(ALL_GO_MOD_DIRS:%=go-mod-tidy/%)
go-mod-tidy/%: DIR=$*
go-mod-tidy/%: crosslink
	@cd $(DIR) && $(GOCMD) mod tidy -compat=1.20

.PHONY: golangci-lint golangci-lint-fix
golangci-lint-fix: ARGS=--fix
golangci-lint-fix: golangci-lint
golangci-lint: go-mod-tidy generate $(ALL_GO_MOD_DIRS:%=golangci-lint/%)
golangci-lint/%: DIR=$*
golangci-lint/%: | $(GOLANGCI_LINT)
	@echo 'golangci-lint $(if $(ARGS),$(ARGS) ,)$(DIR)' \
		&& cd $(DIR) \
		&& $(GOLANGCI_LINT) run --allow-serial-runners --timeout=2m0s $(ARGS)

.PHONY: build
build: go-mod-tidy generate
	CGO_ENABLED=$(CGO_ENABLED) $(GOCMD) build -o otel-go-instrumentation ./cli/...

.PHONY: docker-build
docker-build:
	docker buildx build -t $(IMG_NAME) .

.PHONY: docker-build-base
docker-build-base:
	docker buildx build -t $(IMG_NAME_BASE) --target base .

docker-dev: docker-build-base
	@docker run \
		-it \
		--rm \
		-v "$(REPODIR)":/usr/src/go.opentelemetry.io/auto \
		-w /usr/src/go.opentelemetry.io/auto \
		$(IMG_NAME_BASE) \
		/bin/bash

LIBBPF_VERSION ?= "< 1.5, >= 1.4.7"
LIBBPF_DEST ?= "$(REPODIR)/internal/include/libbpf"
.PHONY: synclibbpf
synclibbpf: | $(SYNCLIBBPF)
	$(SYNCLIBBPF) -version=$(LIBBPF_VERSION) -dest=$(LIBBPF_DEST)

OFFSETS_OUTPUT_FILE="$(REPODIR)/internal/pkg/inject/offset_results.json"
.PHONY: offsets
offsets: | $(OFFSETGEN)
	$(OFFSETGEN) -output=$(OFFSETS_OUTPUT_FILE) -cache=$(OFFSETS_OUTPUT_FILE)

.PHONY: docker-offsets
docker-offsets: docker-build-base
	docker run -e DOCKER_USERNAME=$(DOCKER_USERNAME) -e DOCKER_PASSWORD=$(DOCKER_PASSWORD) --rm -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock -v $(shell pwd):/app $(IMG_NAME_BASE) /bin/sh -c "cd ../app && make offsets"

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

.PHONY: license-header-check
license-header-check:
	@licRes=$$(for f in $$(find . -type f \( -iname '*.go' -o -iname '*.sh' -o -iname '*.c' -o -iname '*.h' \) ! -path '**/third_party/*' ! -path './.git/*' ! -path './LICENSES/*' ! -path './internal/include/libbpf/*' ) ; do \
	           awk '/Copyright The OpenTelemetry Authors|generated|GENERATED/ && NR<=3 { found=1; next } END { if (!found) print FILENAME }' $$f; \
	           awk '/SPDX-License-Identifier: Apache-2.0|generated|GENERATED/ && NR<=4 { found=1; next } END { if (!found) print FILENAME }' $$f; \
	   done); \
	   if [ -n "$${licRes}" ]; then \
	           echo "license header checking failed:"; echo "$${licRes}"; \
	           exit 1; \
	   fi

.PHONY: fixture-nethttp fixture-gin fixture-databasesql fixture-nethttp-custom fixture-otelglobal fixture-autosdk fixture-kafka-go
fixture-nethttp-custom: fixtures/nethttp_custom
fixture-nethttp: fixtures/nethttp
fixture-gin: fixtures/gin
fixture-databasesql: fixtures/databasesql
fixture-grpc: fixtures/grpc
fixture-otelglobal: fixtures/otelglobal
fixture-autosdk: fixtures/autosdk
fixture-kafka-go: fixtures/kafka-go
fixtures/%: LIBRARY=$*
fixtures/%: docker-build
	if [ -f ./internal/test/e2e/$(LIBRARY)/build.sh ]; then \
		./internal/test/e2e/$(LIBRARY)/build.sh; \
	else \
		cd internal/test/e2e/$(LIBRARY) && docker build -t sample-app . ;\
	fi
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
		rm -f ./internal/test/e2e/$(LIBRARY)/traces-orig.json; \
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

# Virtualized python tools via docker

# The directory where the virtual environment is created.
VENVDIR := venv

# The directory where the python tools are installed.
PYTOOLS := $(VENVDIR)/bin

# The pip executable in the virtual environment.
PIP := $(PYTOOLS)/pip

# The directory in the docker image where the current directory is mounted.
WORKDIR := /workdir

# The python image to use for the virtual environment.
PYTHONIMAGE := $(shell awk '$$4=="python" {print $$2}' $(DEPENDENCIES_DOCKERFILE))

# Run the python image with the current directory mounted.
DOCKERPY := docker run --rm -u $(DOCKER_USER) -v "$(CURDIR):$(WORKDIR)" -w $(WORKDIR) $(PYTHONIMAGE)

# Create a virtual environment for Python tools.
$(PYTOOLS):
# The `--upgrade` flag is needed to ensure that the virtual environment is
# created with the latest pip version.
	@$(DOCKERPY) bash -c "python3 -m venv $(VENVDIR) && $(PIP) install --upgrade --cache-dir=$(WORKDIR)/.cache/pip pip"

# Install python packages into the virtual environment.
$(PYTOOLS)/%: $(PYTOOLS)
	@$(DOCKERPY) $(PIP) install --cache-dir=$(WORKDIR)/.cache/pip -r requirements.txt

CODESPELL = $(PYTOOLS)/codespell
$(CODESPELL): PACKAGE=codespell

.PHONY: codespell
codespell: $(CODESPELL)
	@$(DOCKERPY) $(CODESPELL)
