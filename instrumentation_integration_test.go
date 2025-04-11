// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/goleak"

	"go.opentelemetry.io/auto"
)

const (
	batsVerify = "verify.bats"
	tracesOrig = "traces-orig.json"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running integration test in short mode.")
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		t.Skip("cannot manage memory, skipping test.")
	}

	defer goleak.VerifyNone(t)

	t.Run("AutoSDK", testIntegration("./internal/test/e2e/autosdk"))
	t.Run("DatabaseSQL", testIntegration("./internal/test/e2e/databasesql"))
	t.Run("GRPC", testIntegration("./internal/test/e2e/grpc"))
	t.Run("Gin", testIntegration("./internal/test/e2e/gin"))
	t.Run("KafkaGo", testIntegration("./internal/test/e2e/kafka-go"))
	t.Run("NetHTTP", testIntegration("./internal/test/e2e/nethttp"))
	t.Run("NetHTTPCustom", testIntegration("./internal/test/e2e/nethttp_custom"))
	t.Run("OTelGlobal", testIntegration("./internal/test/e2e/otelglobal"))
}

func testIntegration(pkgPath string) func(*testing.T) {
	return func(t *testing.T) {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		binPath := compile(t, ctx, pkgPath)

		server := newCollector(t)
		defer server.Close()

		run(t, ctx, binPath, server.URL)

		server.Flush(t, pkgPath)
		bats(t, ctx, pkgPath)
	}
}

func bats(t *testing.T, ctx context.Context, pkgPath string) {
	cmd := exec.CommandContext(ctx, "bats", batsVerify)
	cmd.Dir = pkgPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run bats: %v", err)
	}
}

func compile(t *testing.T, ctx context.Context, pkgPath string) string {
	t.Helper()

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, filepath.Base(pkgPath))

	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binaryPath, ".")
	cmd.Dir = pkgPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to compile binary: %v", err)
	}

	return binaryPath
}

type collector struct {
	*httptest.Server

	Received ptrace.Traces
}

func newCollector(t *testing.T) *collector {
	c := &collector{Received: ptrace.NewTraces()}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Deserialize the OTLP traces using pdata
		var unmarshaler ptrace.ProtoUnmarshaler
		traces, err := unmarshaler.UnmarshalTraces(body)
		if err != nil {
			t.Errorf("Failed to unmarshal OTLP traces: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		traces.ResourceSpans().MoveAndAppendTo(c.Received.ResourceSpans())

		// Respond with status OK
		w.WriteHeader(http.StatusOK)
	})

	c.Server = httptest.NewServer(mux)
	return c
}

// Flush flushes all received data to the dest directory.
func (c *collector) Flush(t *testing.T, dest string) {
	t.Helper()

	// Create the file in the specified directory
	filePath := filepath.Join(dest, tracesOrig)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create %s: %v", filePath, err)
	}
	defer file.Close()

	// Marshal the data to JSON.
	var marshaler ptrace.JSONMarshaler
	data, err := marshaler.MarshalTraces(c.Received)
	if err != nil {
		t.Fatalf("Failed to marshal traces: %v", err)
	}

	// Write JSON data to the file
	if _, err := file.Write(data); err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
}

func run(t *testing.T, ctx context.Context, binPath string, endpoint string) {
	t.Helper()

	t.Log("Loading target")
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Args = append(cmd.Args, "-trigger=signal:SIGCONT")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	t.Log("Starting target")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start target: %v", err)
	}

	t.Setenv("OTEL_SERVICE_NAME", "sample-app")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	t.Setenv("OTEL_GO_AUTO_INCLUDE_DB_STATEMENT", "true")
	t.Setenv("OTEL_GO_AUTO_PARSE_DB_STATEMENT", "true")

	t.Log("Creating auto-instrumentation")
	inst, err := auto.NewInstrumentation(
		ctx,
		auto.WithPID(cmd.Process.Pid),
		auto.WithGlobal(),
		auto.WithLogger(NewTestLogger(t)),
		auto.WithEnv(),
	)
	if err != nil {
		t.Fatalf("Failed to create auto-instrumentation: %v", err)
	}

	t.Log("Loading")
	if err = inst.Load(ctx); err != nil {
		t.Fatalf("Failed to load auto-instrumentation: %v", err)
	}

	t.Log("Running")
	runCh := make(chan error, 1)
	go func() {
		runCh <- inst.Run(ctx)
		close(runCh)
	}()

	var sig os.Signal = syscall.SIGCONT
	t.Log("Sending signal to target")
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatalf("Failed to send signal to target: %v", err)
	}
	t.Log("Sent signal to target")

	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	func() {
		for {
			select {
			case <-ctx.Done():
				t.Fatal("Context ended")
			case err := <-runCh:
				if err != nil {
					t.Fatalf("Failed to run: %v", err)
				}
				// Do not return. Wait for doneCh.
			case <-doneCh:
				if err != nil {
					t.Fatalf("Application failed: %v", err)
				}
				return
			}
		}
	}()

	t.Log("Closing instrumentation")
	if err := inst.Close(); err != nil {
		t.Fatalf("Failed to close auto-instrumentation: %v", err)
	}
}

// testLogger is an slog.Handler that logs to testing.T.
type testLogger struct {
	t *testing.T
}

// Enabled returns true to log all levels.
func (tl *testLogger) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle logs the record to testing.T.
func (tl *testLogger) Handle(_ context.Context, r slog.Record) error {
	var pcs [1]uintptr
	runtime.Callers(4, pcs[:])
	frame, _ := runtime.CallersFrames(pcs[:]).Next()

	tl.t.Logf(
		"[%s:%d] %s: %s",
		frame.Function,
		frame.Line,
		r.Level.String(),
		r.Message,
	)
	r.Attrs(func(a slog.Attr) bool {
		tl.t.Logf("\t%s = %v", a.Key, a.Value.Resolve())
		return true
	})

	return nil
}

// WithAttrs returns a new handler with added attributes.
func (tl *testLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return tl // Ignore attributes for simplicity.
}

// WithGroup returns a new handler (groups ignored for simplicity).
func (tl *testLogger) WithGroup(_ string) slog.Handler {
	return tl
}

// NewTestLogger returns an *slog.Logger that logs to testing.T.
func NewTestLogger(t *testing.T) *slog.Logger {
	return slog.New(&testLogger{t})
}
