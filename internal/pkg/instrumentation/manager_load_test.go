// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ebpf_test

package instrumentation

import (
	"log/slog"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/auto/internal/pkg/inject"
	dbSql "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/database/sql"
	kafkaConsumer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/consumer"
	kafkaProducer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/github.com/segmentio/kafka-go/producer"
	autosdk "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/auto/sdk"
	otelTraceGlobal "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/go.opentelemetry.io/otel/traceglobal"
	grpcClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/client"
	grpcServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/google.golang.org/grpc/server"
	httpClient "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/client"
	httpServer "go.opentelemetry.io/auto/internal/pkg/instrumentation/bpf/net/http/server"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/testutils"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/process/binary"
)

func TestLoadProbes(t *testing.T) {
	ver := utils.GetLinuxKernelVersion()
	require.NotNil(t, ver)
	t.Logf("Running on kernel %s", ver.String())
	m := fakeManager(t)

	for _, p := range m.probes {
		manifest := p.Manifest()
		fields := manifest.StructFields
		offsets := map[string]*semver.Version{}
		for _, f := range fields {
			_, ver := inject.GetLatestOffset(f)
			if ver != nil {
				offsets[f.PkgPath] = ver
				offsets[f.ModPath] = ver
			}
		}
		t.Run(p.Manifest().ID.String(), func(t *testing.T) {
			testProbe, ok := p.(testutils.TestProbe)
			assert.True(t, ok)
			testutils.ProbesLoad(t, testProbe, offsets)
		})
	}
}

func fakeManager(t *testing.T, fnNames ...string) *Manager {
	logger := slog.Default()
	probes := []probe.Probe{
		grpcClient.New(logger, ""),
		grpcServer.New(logger, ""),
		httpServer.New(logger, ""),
		httpClient.New(logger, ""),
		dbSql.New(logger, ""),
		kafkaProducer.New(logger, ""),
		kafkaConsumer.New(logger, ""),
		autosdk.New(logger),
		otelTraceGlobal.New(logger),
	}
	ver := semver.New(1, 20, 0, "", "")
	var fn []*binary.Func
	for _, name := range fnNames {
		fn = append(fn, &binary.Func{Name: name})
	}
	m := &Manager{
		logger: slog.Default(),
		cp:     NewNoopConfigProvider(nil),
		probes: make(map[probe.ID]probe.Probe),
		proc: &process.Info{
			ID:        1,
			Functions: fn,
			GoVersion: ver,
			Modules:   map[string]*semver.Version{},
		},
	}
	for _, p := range probes {
		m.probes[p.Manifest().ID] = p
	}
	m.filterUnusedProbes()

	return m
}
