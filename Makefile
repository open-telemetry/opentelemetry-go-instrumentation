# Obtain an absolute path to the directory of the Makefile.
# Assume the Makefile is in the root of the repository.
REPODIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Build the list of include directories to compile the bpf program
BPF_INCLUDE += -I${REPODIR}/include/libbpf
BPF_INCLUDE+= -I${REPODIR}/include

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
	cd offsets-tracker; go run main.go
