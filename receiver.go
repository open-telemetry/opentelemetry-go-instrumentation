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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const receiverType = "auto"

var cfgType = component.MustNewType(receiverType)

type Receiver struct {
	logger   *slog.Logger
	resource pcommon.Resource

	nextTracesMu sync.Mutex
	nextTraces   []consumer.Traces
}

var (
	_ receiver.Traces = (*Receiver)(nil)
	_ TraceHandler    = (*Receiver)(nil)
)

func NewReceiver(l *slog.Logger, res pcommon.Resource) *Receiver {
	return &Receiver{logger: l, resource: res}
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
		receiver.WithTraces(createTraces, component.StabilityLevelAlpha),
	)
}

func (*Receiver) Start(context.Context, component.Host) error {
	return nil
}

func (*Receiver) Shutdown(context.Context) error {
	return nil
}

func (r *Receiver) RegisterConsumer(c consumer.Traces) {
	r.logger.Debug("registering consumer")

	r.nextTracesMu.Lock()
	defer r.nextTracesMu.Unlock()

	r.nextTraces = append(r.nextTraces, c)
}

func (r *Receiver) HandleScopeSpans(ctx context.Context, data ptrace.ScopeSpans) error {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	r.resource.CopyTo(rs.Resource())
	rs.SetSchemaUrl(semconv.SchemaURL)
	data.CopyTo(rs.ScopeSpans().AppendEmpty())

	r.nextTracesMu.Lock()
	defer r.nextTracesMu.Unlock()

	r.logger.Debug(
		"handling scope spans",
		"consumers", len(r.nextTraces),
		"spans", traces.SpanCount(),
	)

	var err error
	for _, c := range r.nextTraces {
		err = errors.Join(err, c.ConsumeTraces(ctx, traces))
	}
	return err
}
