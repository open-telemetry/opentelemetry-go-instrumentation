// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe/sampling"
	"go.opentelemetry.io/auto/internal/pkg/process"
)

func TestWithPID(t *testing.T) {
	c, err := newInstConfig(context.Background(), []InstrumentationOption{WithPID(1)})
	require.NoError(t, err)
	assert.Equal(t, process.ID(1), c.pid)
}

func TestWithEnv(t *testing.T) {
	t.Run("OTEL_LOG_LEVEL", func(t *testing.T) {
		orig := newLogger
		var got slog.Leveler
		newLogger = func(level slog.Leveler) *slog.Logger {
			got = level
			return newLoggerFunc(level)
		}
		t.Cleanup(func() { newLogger = orig })

		t.Setenv(envLogLevelKey, "debug")
		ctx, opts := context.Background(), []InstrumentationOption{WithEnv()}
		_, err := newInstConfig(ctx, opts)
		require.NoError(t, err)

		assert.Equal(t, slog.LevelDebug, got)

		t.Setenv(envLogLevelKey, "invalid")
		_, err = newInstConfig(ctx, opts)
		require.ErrorContains(t, err, `parse log level "invalid"`)
	})
}

func TestOptionPrecedence(t *testing.T) {
	const b = "false"

	t.Run("Env", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_GO_AUTO_GLOBAL": b,
		})

		// WithEnv passed last, it should have precedence.
		opts := []InstrumentationOption{
			WithGlobal(),
			WithEnv(),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.False(t, c.globalImpl)
	})

	t.Run("Options", func(t *testing.T) {
		mockEnv(t, map[string]string{
			"OTEL_GO_AUTO_GLOBAL": b,
		})

		// WithEnv passed first, it should be overridden.
		opts := []InstrumentationOption{
			WithEnv(),
			WithGlobal(),
		}
		c, err := newInstConfig(context.Background(), opts)
		require.NoError(t, err)
		assert.True(t, c.globalImpl)
	})
}

func TestWithLogger(t *testing.T) {
	l := slog.New(slog.Default().Handler())
	opts := []InstrumentationOption{WithLogger(l)}
	c, err := newInstConfig(context.Background(), opts)
	require.NoError(t, err)

	assert.Same(t, l, c.logger)
}

func TestWithSampler(t *testing.T) {
	t.Run("Default sampler", func(t *testing.T) {
		c, err := newInstConfig(context.Background(), []InstrumentationOption{})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.DefaultConfig().Samplers, sc.Samplers)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		conf, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, conf.SamplerType)
		pbConfig, ok := conf.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.DefaultParentBasedSampler(), pbConfig)
	})

	t.Run("Env config", func(t *testing.T) {
		mockEnv(t, map[string]string{
			tracesSamplerKey:    samplerNameParentBasedTraceIDRatio,
			tracesSamplerArgKey: "0.42",
		})

		c, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		parentBasedConfig, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, parentBasedConfig.SamplerType)
		pbConfig, ok := parentBasedConfig.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.TraceIDRatioID, pbConfig.Root)
		tidRatio, ok := sc.Samplers[sampling.TraceIDRatioID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerTraceIDRatio, tidRatio.SamplerType)
		config, ok := tidRatio.Config.(sampling.TraceIDRatioConfig)
		assert.True(t, ok)
		expected, _ := sampling.NewTraceIDRatioConfig(0.42)
		assert.Equal(t, expected, config)
	})

	t.Run("Invalid Env config", func(t *testing.T) {
		mockEnv(t, map[string]string{
			tracesSamplerKey:    "invalid",
			tracesSamplerArgKey: "0.42",
		})

		_, err := newInstConfig(context.Background(), []InstrumentationOption{WithEnv()})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown sampler name")
	})

	t.Run("WithSampler", func(t *testing.T) {
		c, err := newInstConfig(context.Background(), []InstrumentationOption{
			WithSampler(ParentBasedSampler{
				Root: TraceIDRatioSampler{Fraction: 0.42},
			}),
		})
		require.NoError(t, err)
		sc, err := convertSamplerToConfig(c.sampler)
		assert.NoError(t, err)
		assert.Equal(t, sampling.ParentBasedID, sc.ActiveSampler)
		parentBasedConfig, ok := sc.Samplers[sampling.ParentBasedID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerParentBased, parentBasedConfig.SamplerType)
		pbConfig, ok := parentBasedConfig.Config.(sampling.ParentBasedConfig)
		assert.True(t, ok)
		assert.Equal(t, sampling.TraceIDRatioID, pbConfig.Root)
		assert.Equal(t, sampling.AlwaysOnID, pbConfig.RemoteSampled)
		assert.Equal(t, sampling.AlwaysOffID, pbConfig.RemoteNotSampled)
		assert.Equal(t, sampling.AlwaysOnID, pbConfig.LocalSampled)
		assert.Equal(t, sampling.AlwaysOffID, pbConfig.LocalNotSampled)

		tidRatio, ok := sc.Samplers[sampling.TraceIDRatioID]
		assert.True(t, ok)
		assert.Equal(t, sampling.SamplerTraceIDRatio, tidRatio.SamplerType)
		config, ok := tidRatio.Config.(sampling.TraceIDRatioConfig)
		assert.True(t, ok)
		expected, _ := sampling.NewTraceIDRatioConfig(0.42)
		assert.Equal(t, expected, config)
	})
}

func mockEnv(t *testing.T, env map[string]string) {
	orig := lookupEnv
	t.Cleanup(func() { lookupEnv = orig })

	lookupEnv = func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}
