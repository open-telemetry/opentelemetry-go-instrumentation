package otelsdk_test

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	"go.opentelemetry.io/auto"
	"go.opentelemetry.io/auto/export/otelsdk"
)

func Example_multiplex() {
	ctx := context.Background()
	m, err := otelsdk.NewMultiplexer(ctx)
	if err != nil {
		panic(err)
	}

	pids := []int{1297, 1331, 9827} // Simulated PIDs

	var wg sync.WaitGroup
	for _, pid := range pids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM)
			defer stop()

			// Note: do not ignore errors in normal use.
			inst, _ := auto.NewInstrumentation(
				ctx, auto.WithPID(id), auto.WithHandler(m.Handler(id)),
			)
			_ = inst.Load(ctx)
			_ = inst.Run(ctx)
		}(pid)
	}

	wg.Wait()
	_ = m.Shutdown(ctx)
}
