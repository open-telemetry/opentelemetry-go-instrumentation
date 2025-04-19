// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsdk_test

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/pipeline/otelsdk"
)

func Example_multiplex() {
	// Create a context that cancels when a SIGTERM is received. This ensures
	// that each instrumentation goroutine below can shut down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	// Create a new multiplexer to handle instrumentation events from multiple
	// sources. This will act as a central router for telemetry handlers.
	m, err := otelsdk.NewMultiplexer(ctx)
	if err != nil {
		panic(err)
	}

	// Simulated process IDs to be instrumented. These would typically be real
	// process IDs in a production scenario.
	pids := []int{1297, 1331, 9827}

	var wg sync.WaitGroup
	for _, pid := range pids {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			// Create a new instrumentation session for the process.
			//
			// NOTE: Error handling is omitted here for brevity. In production
			// code, always check and handle errors.
			inst, _ := auto.NewInstrumentation(
				ctx,
				auto.WithPID(id),
				auto.WithHandler(m.Handler(id)),
			)

			// Load and start the instrumentation for the process.
			_ = inst.Load(ctx)
			_ = inst.Run(ctx)
		}(pid)
	}

	// Wait for all instrumentation goroutines to complete.
	wg.Wait()

	// Shut down the multiplexer, cleaning up any remaining resources.
	_ = m.Shutdown(ctx)
}
