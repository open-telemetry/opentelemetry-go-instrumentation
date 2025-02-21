// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cli runs OpenTelemetry automatic instrumentation for Go packages
// using eBPF.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/export/otelsdk"
)

const help = `Usage of %s:
  -global-impl
    	Record telemetry from the OpenTelemetry default global implementation
  -target-pid int
    	PID of target process
  -target-exe string
    	Executable path run by the target process
  -log-level string
    	Logging level ("debug", "info", "warn", "error")

Runs the OpenTelemetry auto-instrumentation for Go applications using eBPF.

If both -target-pid and -target-exe are provided -target-exe will be ignored
and -target-pid used.

Environment variable configuration:

	- OTEL_GO_AUTO_TARGET_PID: PID of the target process
	- OTEL_GO_AUTO_TARGET_EXE: executable path run by the target process
	- OTEL_LOG_LEVEL: log level (flag takes precedence)
	- OTEL_SERVICE_NAME (or OTEL_RESOURCE_ATTRIBUTES): service name
	- OTEL_TRACES_EXPORTER: trace exporter identifier

If the OTEL_GO_AUTO_TARGET_PID is only resolved if -target-exe or -target-pid
is not provided. If none of these are set, OTEL_GO_AUTO_TARGET_EXE will be
resolved.

The OTEL_TRACES_EXPORTER environment variable value is resolved using the
autoexport (go.opentelemetry.io/contrib/exporters/autoexport) package. See that
package's documentation for information on supported values and registration of
custom exporters.
`

const (
	// envLogLevelKey is the key for the environment variable value containing the
	// log level.
	envLogLevelKey = "OTEL_LOG_LEVEL"
	// envTargetPIDKey is the environment variable key containing the target
	// process ID to instrument.
	envTargetPIDKey = "OTEL_GO_AUTO_TARGET_PID"
	// envTargetExeKey is the environment variable key containing the path to
	// target binary to instrument.
	envTargetExeKey = "OTEL_GO_AUTO_TARGET_EXE"
)

func usage() {
	program := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, help, program)
}

func newLogger(lvlStr string) *slog.Logger {
	levelVar := new(slog.LevelVar) // Default value of info.
	opts := &slog.HandlerOptions{AddSource: true, Level: levelVar}
	h := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(h)

	if lvlStr == "" {
		lvlStr = os.Getenv(envLogLevelKey)
	}

	if lvlStr == "" {
		return logger
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(lvlStr)); err != nil {
		logger.Error("failed to parse log level", "error", err, "log-level", lvlStr)
	} else {
		levelVar.Set(level)
	}

	return logger
}

func main() {
	var globalImpl bool
	var logLevel string
	var targetPID int
	var targetExe string

	flag.BoolVar(&globalImpl, "global-impl", false, "Record telemetry from the OpenTelemetry default global implementation")
	flag.StringVar(&logLevel, "log-level", "", `Logging level ("debug", "info", "warn", "error")`)
	flag.IntVar(&targetPID, "target-pid", -1, `PID of target process`)
	flag.StringVar(&targetExe, "target-exe", "", `Executable path run by the target process`)

	flag.Usage = usage
	flag.Parse()

	logger := newLogger(logLevel)

	// Trap Ctrl+C and SIGTERM and call cancel on the context.
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(ch)
		cancel()
	}()
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()

	logger.Info(
		"building OpenTelemetry Go instrumentation ...",
		"globalImpl", globalImpl,
		"version", newVersion(),
	)

	v := semconv.TelemetryDistroVersionKey.String(auto.Version())
	h, err := otelsdk.New(
		ctx,
		otelsdk.WithEnv(),
		otelsdk.WithLogger(logger),
		otelsdk.WithResourceAttributes(v),
	)
	if err != nil {
		logger.Error("failed to create OTel SDK handler", "error", err)
		return
	}

	instOptions := []auto.InstrumentationOption{
		auto.WithEnv(),
		auto.WithLogger(logger),
		auto.WithHandler(h),
	}
	if globalImpl {
		instOptions = append(instOptions, auto.WithGlobal())
	}
	pid, err := findPID(ctx, logger, targetPID, targetExe)
	if err != nil {
		logger.Error("failed to find target", "error", err)
		return
	}
	instOptions = append(instOptions, auto.WithPID(pid))

	logger.Info(
		"building OpenTelemetry Go instrumentation ...",
		"globalImpl", globalImpl,
		"PID", pid,
		"version", newVersion(),
	)

	inst, err := auto.NewInstrumentation(ctx, instOptions...)
	if err != nil {
		logger.Error("failed to create instrumentation", "error", err)
		return
	}

	err = inst.Load(ctx)
	if err != nil {
		logger.Error("failed to load instrumentation", "error", err)
		return
	}

	logger.Info("instrumentation loaded successfully, starting...")

	if err = inst.Run(ctx); err != nil {
		logger.Error("instrumentation crashed", "error", err)
	}

	logger.Info("shutting down")

	ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err = h.Shutdown(ctx)
	if err != nil {
		logger.Error("failed to flush handler", "error", err)
	}
}

var errNoPID = fmt.Errorf(
	"no target: -target-pid or -target-exe not provided and the env vars %s and %s are unset",
	envTargetPIDKey, envTargetExeKey,
)

func findPID(ctx context.Context, l *slog.Logger, pid int, binPath string) (int, error) {
	// Priority:
	//  1. pid
	//  2. binPath
	//  3. OTEL_GO_AUTO_TARGET_PID
	//  4. OTEL_GO_AUTO_TARGET_EXE

	l.Debug(
		"finding target PID",
		"PID", pid,
		"executable", binPath,
		envTargetPIDKey, os.Getenv(envTargetPIDKey),
		envTargetExeKey, os.Getenv(envTargetExeKey),
	)

	if pid >= 0 {
		return pid, nil
	}

	if binPath != "" {
		return findExeFn(ctx, l, binPath)
	}

	pidStr := os.Getenv(envTargetPIDKey)
	if pidStr != "" {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return 0, fmt.Errorf("invalid OTEL_GO_AUTO_TARGET_PID value: %s: %w", pidStr, err)
		}
		return pid, nil
	}

	binPath = os.Getenv(envTargetExeKey)
	if binPath != "" {
		return findExeFn(ctx, l, binPath)
	}

	return -1, errNoPID
}

// Used for testing.
var findExeFn = findExe

func findExe(ctx context.Context, l *slog.Logger, exe string) (int, error) {
	pp := ProcessPoller{Logger: l, BinPath: exe}
	return pp.Poll(ctx)
}
