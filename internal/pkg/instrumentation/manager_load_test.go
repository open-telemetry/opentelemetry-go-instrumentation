// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/database/sql"
	kafkaConsumer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/consumer"
	kafkaProducer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/producer"
	autosdk "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/auto/sdk"
	otelTraceGlobal "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/otel/traceglobal"
	grpcClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client"
	grpcServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/server"
	httpClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/client"
	httpServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/server"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/bpffs"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

func TestLoadProbes(t *testing.T) {
	if err := rlimit.RemoveMemlock(); err != nil {
		t.Skip("cannot manage memory, skipping test.")
	}

	id := setupTestModule(t)
	pid := process.ID(id)

	logger := slog.Default()
	info, err := process.NewInfo(logger, pid, make(map[string]interface{}))
	if info == nil {
		t.Fatalf("failed to create process.Info: %v", err)
	}
	// Reset Info module information.
	info.Modules = make(map[string]*semver.Version)

	ver := utils.GetLinuxKernelVersion()
	require.NotNil(t, ver)
	t.Logf("Running on kernel %s", ver.String())

	probes := []probe.Probe{
		grpcClient.New(logger, ""),
		grpcServer.New(logger, ""),
		httpServer.New(logger, ""),
		httpClient.New(logger, ""),
		dbSql.New(logger, ""),
		kafkaProducer.New(logger, ""),
		kafkaConsumer.New(logger, ""),
		autosdk.New(logger),
		otelTraceGlobal.New(logger),
	}

	for _, p := range probes {
		manifest := p.Manifest()
		fields := manifest.StructFields
		for _, f := range fields {
			_, ver := inject.GetLatestOffset(f)
			if ver != nil {
				info.Modules[f.PkgPath] = ver
				info.Modules[f.ModPath] = ver
			}
		}
		t.Run(p.Manifest().ID.String(), func(t *testing.T) {
			require.Implements(t, (*TestProbe)(nil), p)
			ProbesLoad(t, info, p.(TestProbe))
		})
	}

	// The grpcClient, grpcServer, httpClient, dbSql, kafkaProducer,
	// kafkaConsumer, autosdk, and otelTraceGlobal all allocate. Ensure it has
	// been called.
	a, err := info.Alloc(logger)
	require.NoError(t, err)
	assert.NotEmpty(t, a.StartAddr, "memory not allocated")
}

const mainGoContent = `package main

import (
	"time"
)

func main() {
	for {
		time.Sleep(time.Hour)
	}
}`

func setupTestModule(t *testing.T) int {
	t.Helper()

	tempDir := t.TempDir()

	// Initialize a Go module
	cmd := exec.Command("go", "mod", "init", "example.com/testmodule")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize Go module: %v", err)
	}

	mainGoPath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(mainGoPath, []byte(mainGoContent), 0o600); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Compile the Go program
	binaryPath := filepath.Join(tempDir, "testbinary")
	cmd = exec.Command("go", "build", "-o", binaryPath, mainGoPath)
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile binary: %v", err)
	}

	// Run the compiled binary
	cmd = exec.Command(binaryPath)
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start binary: %v", err)
	}

	// Ensure the process is killed when the test ends
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Return the process ID
	return cmd.Process.Pid
}

type TestProbe interface {
	Spec() (*ebpf.CollectionSpec, error)
	InjectConsts(*process.Info, *ebpf.CollectionSpec) error
}

func ProbesLoad(t *testing.T, info *process.Info, p TestProbe) {
	t.Helper()

	require.NoError(t, bpffs.Mount(info))
	t.Cleanup(func() { _ = bpffs.Cleanup(info) })

	spec, err := p.Spec()
	require.NoError(t, err)

	// Inject the same constants as the BPF program. It is important to inject
	// the same constants as those that will be used in the actual run, since
	// From Linux 5.5 the verifier will use constants to eliminate dead code.
	require.NoError(t, p.InjectConsts(info, spec))

	opts := ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: bpffs.PathForTargetApplication(info),
		},
	}

	collectVerifierLogs := utils.ShouldShowVerifierLogs()
	if collectVerifierLogs {
		opts.Programs.LogLevel = ebpf.LogLevelStats | ebpf.LogLevelInstruction
	}

	c, err := ebpf.NewCollectionWithOptions(spec, opts)
	if !assert.NoError(t, err) {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) && collectVerifierLogs {
			t.Logf("Verifier log: %-100v\n", ve)
		}
	}

	if c != nil {
		t.Cleanup(c.Close)
	}
}
