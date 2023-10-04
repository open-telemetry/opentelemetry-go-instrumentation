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
	"path/filepath"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"

	"go.opentelemetry.io/auto/internal/pkg/log"
	"go.opentelemetry.io/auto/internal/pkg/orchestrator"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

const (
	// envTargetExeKey is the key for the environment variable value pointing to the
	// target binary to instrument.
	envTargetExeKey = "OTEL_GO_AUTO_TARGET_EXE"
	// envServiceName is the key for the envoriment variable value containing the service name.
	envServiceNameKey = "OTEL_SERVICE_NAME"
	// envResourceAttrKey is the key for the environment variable value containing
	// OpenTelemetry Resource attributes.
	envResourceAttrKey = "OTEL_RESOURCE_ATTRIBUTES"
	// serviceNameDefault is the default service name prefix used if a user does not provide one.
	serviceNameDefault = "unknown_service"
)

// Instrumentation manages and controls all OpenTelemetry Go
// auto-instrumentation.
type Instrumentation struct {
	target       *process.TargetArgs
	orchestrator *orchestrator.Service
}

// Error message returned when instrumentation is launched without a target
// binary.
var errUndefinedTarget = fmt.Errorf("undefined target Go binary, consider setting the %s environment variable pointing to the target binary to instrument", envTargetExeKey)

// NewInstrumentation returns a new [Instrumentation] configured with the
// provided opts.
func NewInstrumentation(opts ...InstrumentationOption) (*Instrumentation, error) {
	c := newInstConfig(opts)
	if err := c.validate(); err != nil {
		return nil, err
	}

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
	targetArgs := process.ParseTargetArgs()
	if targetArgs != nil {
		if err := targetArgs.Validate(); err != nil {
			log.Logger.Error(err, "invalid target args")
			return nil, err
		}
	}
	r, err := orchestrator.New(ctx, targetArgs, traceExporter)
	if err != nil {
		log.Logger.V(0).Error(err, "creating orchestrator")
	}

	return &Instrumentation{
		target:       targetArgs,
		orchestrator: r,
	}, nil
}

// Run starts the instrumentation.
func (i *Instrumentation) Run() error {
	return i.orchestrator.Run()
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
	target      *process.TargetArgs
	serviceName string
}

func newInstConfig(opts []InstrumentationOption) instConfig {
	var c instConfig
	for _, opt := range opts {
		c = opt.apply(c)
	}
	c = c.applyEnv()
	return c
}

func (c instConfig) applyEnv() instConfig {
	if v, ok := os.LookupEnv(envTargetExeKey); ok {
		c.target = &process.TargetArgs{ExePath: v}
	}
	if v, ok := os.LookupEnv(envServiceNameKey); ok {
		c.serviceName = v
	} else {
		c = c.applyResourceAtrrEnv()
		if c.serviceName == "" {
			c = c.setDefualtServiceName()
		}
	}
	return c
}

func (c instConfig) setDefualtServiceName() instConfig {
	if c.target != nil {
		c.serviceName = fmt.Sprintf("%s:%s", serviceNameDefault, filepath.Base(c.target.ExePath))
	} else {
		c.serviceName = serviceNameDefault
	}
	return c
}

func (c instConfig) applyResourceAtrrEnv() instConfig {
	attrs := strings.TrimSpace(os.Getenv(envResourceAttrKey))

	if attrs == "" {
		return c
	}

	pairs := strings.Split(attrs, ",")
	for _, p := range pairs {
		k, v, found := strings.Cut(p, "=")
		if !found {
			continue
		}
		key := strings.TrimSpace(k)
		if key == string(semconv.ServiceNameKey) {
			c.serviceName = strings.TrimSpace(v)
		}
	}

	return c
}

func (c instConfig) validate() error {
	if c.target == nil {
		return errUndefinedTarget
	}
	return c.target.Validate()
}

type fnOpt func(instConfig) instConfig

func (o fnOpt) apply(c instConfig) instConfig { return o(c) }

// WithTarget returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being executed at the provided path.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined it will take precedence over any value
// passed here.
func WithTarget(path string) InstrumentationOption {
	return fnOpt(func(c instConfig) instConfig {
		c.target = &process.TargetArgs{ExePath: path}
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
