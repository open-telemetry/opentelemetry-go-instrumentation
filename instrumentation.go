// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auto

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"google.golang.org/grpc"

	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/orchestrator"
)

var (
	// Start of this auto-instrumentation's exporter User-Agent header, e.g. ""OTel-Go-Auto-Instrumentation/1.2.3".
	baseUserAgent = fmt.Sprintf("OTel-Go-Auto-Instrumentation/%s", Version())
	// Information about the runtime environment for inclusion in User-Agent, e.g. "go/1.18.2 (linux/amd64)".
	runtimeInfo = fmt.Sprintf(
		"%s (%s/%s)",
		strings.Replace(runtime.Version(), "go", "go/", 1),
		runtime.GOOS,
		runtime.GOARCH,
	)
	// Combined User-Agent identifying this auto-instrumentation and its runtime environment, see RFC7231 for format considerations.
	autoinstUserAgent = fmt.Sprintf("%s %s", baseUserAgent, runtimeInfo)
)

// Instrumentation manages and controls all OpenTelemetry Go
// auto-instrumentation.
type Instrumentation struct {
	orchestrator *orchestrator.Service
	ctx          context.Context
}

// NewInstrumentation returns a new [Instrumentation] configured with the
// provided opts.
func NewInstrumentation(opts ...InstrumentationOption) (*Instrumentation, error) {
	if log.Logger.IsZero() {
		err := log.Init()
		if err != nil {
			return nil, err
		}
	}

	c := newInstConfig(opts)

	ctx := contextWithSigterm(context.Background())
	log.Logger.V(0).Info("Establishing connection to OTLP receiver ...")
	otlpTraceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithDialOption(grpc.WithUserAgent(autoinstUserAgent)),
	)
	traceExporter, err := otlptrace.New(ctx, otlpTraceClient)
	if err != nil {
		log.Logger.Error(err, "unable to connect to OTLP endpoint")
		return nil, err
	}
	r, err := orchestrator.New(
		orchestrator.WithServiceName(c.serviceName),
		orchestrator.WithTarget(c.exePath),
		orchestrator.WithExporter(traceExporter),
		orchestrator.WithVersion(Version()),
		orchestrator.WithPID(c.pid),
	)
	if err != nil {
		log.Logger.V(0).Error(err, "creating orchestrator")
	}

	if err := r.Validate(); err != nil {
		return nil, err
	}

	return &Instrumentation{
		orchestrator: r,
		ctx:          ctx,
	}, nil
}

// Run starts the instrumentation.
func (i *Instrumentation) Run() error {
	return i.orchestrator.Run(i.ctx)
}

func contextWithSigterm(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer close(ch)
		defer signal.Stop(ch)

		select {
		case <-parent.Done(): // if parent is cancelled, return
			return
		case <-ch: // if SIGTERM is received, cancel this context
			cancel()
		}
	}()

	return ctx
}

// InstrumentationOption applies a configuration option to [Instrumentation].
type InstrumentationOption interface {
	apply(instConfig) instConfig
}

type instConfig struct {
	exePath     string
	serviceName string
	pid         int
}

func newInstConfig(opts []InstrumentationOption) instConfig {
	c := instConfig{}
	for _, opt := range opts {
		if opt != nil {
			c = opt.apply(c)
		}
	}
	return c
}

type fnOpt func(instConfig) instConfig

func (o fnOpt) apply(c instConfig) instConfig { return o(c) }

// WithTarget returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being executed at the provided path.
//
// This option conflicts with [WithPID]. If both are used, the last one
// provided to an [Instrumentation] will be used.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined it will take precedence over any value
// passed here.
func WithTarget(path string) InstrumentationOption {
	return fnOpt(func(c instConfig) instConfig {
		c.exePath = path
		return c
	})
}

// WithServiceName returns an [InstrumentationOption] defining the name of the service running.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_SERVICE_NAME is defined it will take precedence over any value
// passed here.
func WithServiceName(serviceName string) InstrumentationOption {
	return fnOpt(func(c instConfig) instConfig {
		c.serviceName = serviceName
		return c
	})
}

// WithPID returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being run with the provided PID.
//
// This option conflicts with [WithTarget]. If both are used, the last one
// provided to an [Instrumentation] will be used.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined it will take precedence over any value
// passed here.
func WithPID(pid int) InstrumentationOption {
	return fnOpt(func(c instConfig) instConfig {
		c.pid = pid
		return c
	})
}
