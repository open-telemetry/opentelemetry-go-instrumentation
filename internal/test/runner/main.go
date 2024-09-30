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

	"github.com/MrAlias/collex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/auto"
)

func main() {
	logLevel := flag.String("log-level", "debug", `logging level ("debug", "info", "warn", "error")`)
	binPath := flag.String("bin", "", "Path to the target binary")
	outPath := flag.String("out", "traces.json", "Path to out generated traces")
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
	if err := app.Run(ctx, *binPath, *outPath); err != nil {
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

func (a *App) Run(ctx context.Context, binPath, outPath string) error {
	exp, err := a.newExporter(ctx, outPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(binPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	loadedIndicator := make(chan struct{})
	inst, err := auto.NewInstrumentation(
		ctx,
		auto.WithTraceExporter(exp),
		auto.WithPID(cmd.Process.Pid),
		auto.WithServiceName("testing"),
		auto.WithGlobal(),
		auto.WithLoadedIndicator(loadedIndicator),
		auto.WithLogger(a.logger),
	)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- inst.Run(ctx)
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-loadedIndicator:
	}

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

	return inst.Close()
}

func (a *App) newExporter(ctx context.Context, outPath string) (trace.SpanExporter, error) {
	f := fileexporter.NewFactory()
	factory, err := collex.NewFactory(f, nil)
	if err != nil {
		return nil, err
	}
	c := f.CreateDefaultConfig().(*fileexporter.Config)
	c.Path = outPath
	return factory.SpanExporter(ctx, c)
}
