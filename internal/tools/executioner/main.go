// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	telemetryPath = "/metrics"
	telemetryPort = 8888
	shutdownPath  = "/shutdown"
	shutdownPort  = 8080
	metricName    = "otelcol_exporter_sent_spans"
	exporterAttr  = `exporter="file/trace"`
)

func main() {
	// Command-line flags
	collectorAddress := flag.String("collector-address", "http://collector", "Address of the collector")
	spanCount := flag.Int("span-count", 0, "Number of spans to check before shutting down the collector")
	checkLimit := flag.Int("limit", 5, "Maximum number of times to check the span count")
	interval := flag.Duration("interval", 2*time.Second, "Duration between span count checks")
	flag.Parse()

	if *spanCount < 0 {
		fmt.Println("Error: span-count must not be negative")
		return
	}

	telemetryURL := fmt.Sprintf("%s:%d%s", *collectorAddress, telemetryPort, telemetryPath)
	shutdownURL := fmt.Sprintf("%s:%d%s", *collectorAddress, shutdownPort, shutdownPath)

	// Context to handle SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *spanCount == 0 {
		fmt.Println("Span count is 0. Skipping span count check.")
		if err := sendShutdownSignal(ctx, shutdownURL); err != nil {
			fmt.Println(err)
		}
		return
	}

	fmt.Printf("Checking collector at %s for %d spans\n", telemetryURL, *spanCount)

	for i := 0; i < *checkLimit; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("Received termination signal. Exiting.")
			if err := sendShutdownSignal(ctx, shutdownURL); err != nil {
				fmt.Println(err)
			}
			return
		default:
		}

		spanCountReached, err := checkSpanCount(ctx, telemetryURL, *spanCount)
		if err != nil {
			fmt.Printf("Error checking span count: %v\n", err)
			time.Sleep(*interval)
			continue
		}

		if spanCountReached {
			fmt.Printf("Span count of %d reached.\n", *spanCount)
			if err := sendShutdownSignal(ctx, shutdownURL); err != nil {
				fmt.Println(err)
			}
			return
		}

		time.Sleep(*interval) // Wait before checking again
	}

	fmt.Println("Reached check limit without meeting span count requirement.")
	if err := sendShutdownSignal(ctx, shutdownURL); err != nil {
		fmt.Println(err)
	}
}

func checkSpanCount(ctx context.Context, url string, targetCount int) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to fetch telemetry data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read telemetry response body: %v", err)
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && strings.Contains(line, exporterAttr) {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			var value int
			_, err := fmt.Sscanf(fields[len(fields)-1], "%d", &value)
			if err != nil {
				return false, fmt.Errorf("failed to parse span count: %v", err)
			}

			return value >= targetCount, nil
		}
	}

	return false, nil
}

func sendShutdownSignal(ctx context.Context, url string) error {
	fmt.Printf("Sending shutdown signal to %s\n", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create shutdown request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send shutdown request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fmt.Println("Shutdown signal sent successfully.")
	return nil
}
