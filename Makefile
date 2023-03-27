# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Build the list of include directories to compile the bpf program
BPF_INCLUDE += -I${REPODIR}/include/libbpf
BPF_INCLUDE+= -I${REPODIR}/include

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

.PHONY: generate
generate: export CFLAGS := $(BPF_INCLUDE)
generate:
	go mod tidy
	go generate ./...

.PHONY: build
build: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o otel-go-instrumentation cli/main.go

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: offsets
offsets:
	cd offsets-tracker; OFFSETS_OUTPUT_FILE="../pkg/inject/offset_results.json" go run main.go

.PHONY: update-licenses
update-licenses: | $(GOLICENSES)
	rm -rf LICENSES
	$(GOLICENSES) save ./cli/ --save_path LICENSES
	cp -R ./include/libbpf ./LICENSES

.PHONY: verify-licenses
verify-licenses: | $(GOLICENSES)
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
