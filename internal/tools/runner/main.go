// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

func main() {
	logLevel := flag.String("log-level", "debug", `logging level ("debug", "info", "warn", "error")`)
	binPath := flag.String("bin", "", "Path to the target binary")
	flag.Parse()

	logger := newLogger(*logLevel)

	if *binPath == "" {
		logger.Error("Missing target binary. Please provide a target binary path using the -bin flag")
		os.Exit(1)
	}

	// Trap Ctrl+C and SIGTERM and call cancel on the context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := App{logger: logger}
	if err := app.Run(ctx, *binPath); err != nil {
		logger.Error("failed to run", "error", err)
		os.Exit(1)
	}
}

func newLogger(lvlStr string) *slog.Logger {
	levelVar := new(slog.LevelVar) // Default value of info.
	opts := &slog.HandlerOptions{AddSource: true, Level: levelVar}
	h := slog.NewJSONHandler(os.Stderr, opts)
	logger := slog.New(h)

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

type App struct {
	logger *slog.Logger
}

func (a *App) Run(ctx context.Context, binPath string) error {
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("loading target")
	cmd := exec.Command(binPath)
	cmd.Args = append(cmd.Args, "-trigger=signal:SIGCONT")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	a.logger.Debug("starting target")
	if err := cmd.Start(); err != nil {
		return err
	}

	a.logger.Debug("creating instrumentation")
	inst, err := auto.NewInstrumentation(
		ctx,
		auto.WithTraceExporter(exp),
		auto.WithPID(cmd.Process.Pid),
		auto.WithServiceName("testing"),
		auto.WithGlobal(),
		auto.WithLogger(a.logger),
		auto.WithEnv(),
	)
	if err != nil {
		return err
	}

	a.logger.Debug("loading")
	err = inst.Load(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("running")
	errCh := make(chan error, 1)
	go func() {
		errCh <- inst.Run(ctx)
		close(errCh)
	}()

	var sig os.Signal = syscall.SIGCONT
	a.logger.Debug("sending signal to target")
	cmd.Process.Signal(sig)
	a.logger.Debug("sent signal to target")

	done := make(chan struct{})
	go func() {
		cmd.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
	}

	a.logger.Debug("closing instrumentation")
	return inst.Close()
}
