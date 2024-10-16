// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auto

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
)

const receiverType = "auto"

var cfgType = component.MustNewType(receiverType)

type Receiver struct {
	logger *slog.Logger

	nextTracesMu sync.Mutex
	nextTraces   []consumer.Traces
}

var (
	_ receiver.Traces = (*Receiver)(nil)
	_ TraceHandler    = (*Receiver)(nil)
)

func NewReceiver(l *slog.Logger) *Receiver {
	return &Receiver{logger: l}
}

func (r *Receiver) Factory() receiver.Factory {
	createTraces := func(
		_ context.Context,
		_ receiver.Settings,
		_ component.Config,
		nextConsumer consumer.Traces,
	) (receiver.Traces, error) {
		r.RegisterConsumer(nextConsumer)
		return r, nil
	}

	return receiver.NewFactory(
		cfgType,
		func() component.Config { return nil },
		receiver.WithTraces(createTraces, component.StabilityLevelBeta),
	)
}

func (*Receiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (*Receiver) Shutdown(ctx context.Context) error {
	return nil
}

func (r *Receiver) RegisterConsumer(c consumer.Traces) {
	r.nextTracesMu.Lock()
	defer r.nextTracesMu.Unlock()

	r.nextTraces = append(r.nextTraces, c)
}

func (r *Receiver) HandleScopeSpans(ctx context.Context, data ptrace.ScopeSpans) error {
	traces := ptrace.NewTraces()
	// Copy a resource here
	rs := traces.ResourceSpans().AppendEmpty()
	data.CopyTo(rs.ScopeSpans().AppendEmpty())

	r.nextTracesMu.Lock()
	defer r.nextTracesMu.Unlock()

	var err error
	for _, c := range r.nextTraces {
		err = errors.Join(err, c.ConsumeTraces(ctx, traces))
	}
	return err
}
