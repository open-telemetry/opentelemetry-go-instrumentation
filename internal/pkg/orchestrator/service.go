// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"go.opentelemetry.io/auto/internal/pkg/instrumentors"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

// Service is responsible for managing all instrumentation.
type Service struct {
	ctx         context.Context
	version     string
	analyzer    *process.Analyzer
	exePath     string
	serviceName string
	monitorAll  bool
	processch   chan *pidServiceName
	deadProcess chan int
	managers    map[int]*instrumentors.Manager
	exporter    sdktrace.SpanExporter
	pid         int
	pidTicker   <-chan time.Time
}

type ServiceOpt interface {
	apply(Service) Service
}

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

func (s Service) applyEnv() Service {
	if v, ok := os.LookupEnv(envTargetExeKey); ok {
		s.exePath = v
	}
	if v, ok := os.LookupEnv(envServiceNameKey); ok {
		s.serviceName = v
	} else {
		s = s.applyResourceAtrrEnv()
		if s.serviceName == "" {
			s = s.setDefualtServiceName()
		}
	}
	return s
}

func (s Service) setDefualtServiceName() Service {
	if s.exePath != "" {
		s.serviceName = fmt.Sprintf("%s:%s", serviceNameDefault, filepath.Base(s.exePath))
	} else {
		s.serviceName = serviceNameDefault
	}
	return s
}

func (s Service) applyResourceAtrrEnv() Service {
	attrs := strings.TrimSpace(os.Getenv(envResourceAttrKey))
	serviceName := serviceNameFromAttrs(attrs)
	if serviceName != "" {
		s.serviceName = serviceName
	}
	return s
}

func serviceNameFromAttrs(attrs string) string {
	serviceName := ""
	if attrs == "" {
		return serviceName
	}

	pairs := strings.Split(attrs, ",")
	for _, p := range pairs {
		k, v, found := strings.Cut(p, "=")
		if !found {
			continue
		}
		key := strings.TrimSpace(k)
		if key == string(semconv.ServiceNameKey) {
			serviceName = strings.TrimSpace(v)
		}
	}
	return serviceName
}

func (s Service) Validate() error {
	if s.pid != 0 {
		return validatePID(s.pid)
	}

	if s.monitorAll {
		return nil
	}
	if s.serviceName == "" {
		return fmt.Errorf("serviceName is nil")
	}

	if s.exePath == "" {
		return fmt.Errorf("execPath is nil")
	}

	return nil
}

func validatePID(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("can't find process with pid %d", pid)
	}
	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return fmt.Errorf("process with pid %d does not exist", pid)
	}
	return nil
}

type fnOpt func(Service) Service

func (o fnOpt) apply(c Service) Service { return o(c) }

// WithTarget returns an [InstrumentationOption] defining the target binary for
// [Instrumentation] that is being executed at the provided path.
//
// If multiple of these options are provided to an [Instrumentation], the last
// one will be used.
//
// If OTEL_GO_AUTO_TARGET_EXE is defined it will take precedence over any value
// passed here.
func WithTarget(path string) ServiceOpt {
	return fnOpt(func(c Service) Service {
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
func WithServiceName(serviceName string) ServiceOpt {
	return fnOpt(func(c Service) Service {
		c.serviceName = serviceName
		return c
	})
}

func WithMonitorAll(monitorAll bool) ServiceOpt {
	return fnOpt(func(c Service) Service {
		c.monitorAll = monitorAll
		return c
	})
}

func WithExporter(expoter sdktrace.SpanExporter) ServiceOpt {
	return fnOpt(func(c Service) Service {
		c.exporter = expoter
		return c
	})
}

func WithPID(pid int) ServiceOpt {
	return fnOpt(func(c Service) Service {
		c.pid = pid
		return c
	})
}

func WithVersion(version string) ServiceOpt {
	return fnOpt(func(c Service) Service {
		c.version = version
		return c
	})
}
