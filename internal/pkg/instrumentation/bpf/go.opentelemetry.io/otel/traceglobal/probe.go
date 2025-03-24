// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package global provides an instrumentation probe for the
// [go.opentelemetry.io/otel] global trace API.
package global

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/probe"
	"go.opentelemetry.io/auto/internal/pkg/instrumentation/utils"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf/perf"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/auto/internal/pkg/instrumentation/context"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64,arm64 bpf ./bpf/probe.bpf.c

const (
	// pkg is the package being instrumented.
	pkg = "go.opentelemetry.io/otel/internal/global"

	// Minimum version of go.opentelemetry.io/otel that supports using the
	// go.opentelemetry.io/auto/sdk in the global API.
	minAutoSDK = "1.33.0"
)

func must(c *semver.Constraints, err error) *semver.Constraints {
	if err != nil {
		panic(err)
	}
	return c
}

var (
	goMapsVersion = semver.New(1, 24, 0, "", "")

	otelWithAutoSDK = probe.PackageConstraints{
		Package:     "go.opentelemetry.io/otel",
		Constraints: must(semver.NewConstraint(">= " + minAutoSDK)),
		FailureMode: probe.FailureModeIgnore,
	}
	otelWithoutAutoSDK = probe.PackageConstraints{
		Package:     "go.opentelemetry.io/otel",
		Constraints: must(semver.NewConstraint("< " + minAutoSDK)),
		FailureMode: probe.FailureModeIgnore,
	}
	goWithoutSwissMaps = probe.PackageConstraints{
		Package:     "std",
		Constraints: must(semver.NewConstraint("< " + goMapsVersion.String())),
		// Warn in logs that this is not supported.
		FailureMode: probe.FailureModeWarn,
	}
)

// New returns a new [probe.Probe].
func New(logger *slog.Logger) probe.Probe {
	id := probe.ID{
		SpanKind:        trace.SpanKindClient,
		InstrumentedPkg: pkg,
	}

	uprobeNewStart := &probe.Uprobe{
		Sym:        "go.opentelemetry.io/otel/internal/global.(*tracer).newSpan",
		EntryProbe: "uprobe_newStart",
		PackageConstraints: []probe.PackageConstraints{
			otelWithAutoSDK,
		},
	}

	c := &converter{
		logger:         logger,
		uprobeNewStart: uprobeNewStart,
	}
	return &probe.TraceProducer[bpfObjects, event]{
		Base: probe.Base[bpfObjects, event]{
			ID:     id,
			Logger: logger,
			Consts: []probe.Const{
				probe.AllocationConst{},
				probe.KeyValConst{
					Key: "attr_type_invalid",
					Val: uint64(attribute.INVALID),
				},
				probe.KeyValConst{
					Key: "attr_type_bool",
					Val: uint64(attribute.BOOL),
				},
				probe.KeyValConst{
					Key: "attr_type_int64",
					Val: uint64(attribute.INT64),
				},
				probe.KeyValConst{
					Key: "attr_type_float64",
					Val: uint64(attribute.FLOAT64),
				},
				probe.KeyValConst{
					Key: "attr_type_string",
					Val: uint64(attribute.STRING),
				},
				probe.KeyValConst{
					Key: "attr_type_boolslice",
					Val: uint64(attribute.BOOLSLICE),
				},
				probe.KeyValConst{
					Key: "attr_type_int64slice",
					Val: uint64(attribute.INT64SLICE),
				},
				probe.KeyValConst{
					Key: "attr_type_float64slice",
					Val: uint64(attribute.FLOAT64SLICE),
				},
				probe.KeyValConst{
					Key: "attr_type_stringslice",
					Val: uint64(attribute.STRINGSLICE),
				},
				probe.StructFieldConst{
					Key: "tracer_delegate_pos",
					ID:  structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracer", "delegate"),
				},
				probe.StructFieldConst{
					Key: "tracer_name_pos",
					ID:  structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracer", "name"),
				},
				probe.StructFieldConst{
					Key: "tracer_provider_pos",
					ID:  structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracer", "provider"),
				},
				probe.StructFieldConst{
					Key: "tracer_provider_tracers_pos",
					ID:  structfield.NewID("go.opentelemetry.io/otel", "go.opentelemetry.io/otel/internal/global", "tracerProvider", "tracers"),
				},
				probe.StructFieldConstMaxVersion{
					StructField: probe.StructFieldConst{
						Key: "buckets_ptr_pos",
						ID:  structfield.NewID("std", "runtime", "hmap", "buckets"),
					},
					MaxVersion: goMapsVersion,
				},
				tracerIDContainsSchemaURL{},
				tracerIDContainsScopeAttributes{},
			},
			Uprobes: []*probe.Uprobe{
				uprobeNewStart,
				{
					Sym:         "go.opentelemetry.io/otel/internal/global.(*tracer).Start",
					EntryProbe:  "uprobe_Start",
					ReturnProbe: "uprobe_Start_Returns",
					PackageConstraints: []probe.PackageConstraints{
						otelWithoutAutoSDK,
						goWithoutSwissMaps,
					},
				},
				{
					Sym:        "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).End",
					EntryProbe: "uprobe_End",
					PackageConstraints: []probe.PackageConstraints{
						otelWithoutAutoSDK,
						goWithoutSwissMaps,
					},
				},
				{
					Sym:         "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetAttributes",
					EntryProbe:  "uprobe_SetAttributes",
					FailureMode: probe.FailureModeIgnore,
					PackageConstraints: []probe.PackageConstraints{
						otelWithoutAutoSDK,
						goWithoutSwissMaps,
					},
				},
				{
					Sym:         "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetStatus",
					EntryProbe:  "uprobe_SetStatus",
					FailureMode: probe.FailureModeIgnore,
					PackageConstraints: []probe.PackageConstraints{
						otelWithoutAutoSDK,
						goWithoutSwissMaps,
					},
				},
				{
					Sym:         "go.opentelemetry.io/otel/internal/global.(*nonRecordingSpan).SetName",
					EntryProbe:  "uprobe_SetName",
					FailureMode: probe.FailureModeIgnore,
					PackageConstraints: []probe.PackageConstraints{
						otelWithoutAutoSDK,
						goWithoutSwissMaps,
					},
				},
			},
			SpecFn:        loadBpf,
			ProcessRecord: c.decodeEvent,
		},
		ProcessFn: processFn,
	}
}

type recordKind uint64

const (
	recordKindTelemetry recordKind = iota
	recordKindConrol
)

type converter struct {
	logger *slog.Logger

	uprobeNewStart *probe.Uprobe
}

func (c *converter) decodeEvent(record perf.Record) (*event, error) {
	reader := bytes.NewReader(record.RawSample)

	var kind recordKind
	err := binary.Read(reader, binary.LittleEndian, &kind)
	if err != nil {
		return nil, err
	}

	var e *event
	switch kind {
	case recordKindTelemetry:
		e = new(event)
		err = binary.Read(reader, binary.LittleEndian, e)
	case recordKindConrol:
		if c.uprobeNewStart != nil {
			err = c.uprobeNewStart.Close()
			c.uprobeNewStart = nil
		}
	default:
		err = fmt.Errorf("unknown record kind: %d", kind)
	}
	return e, err
}

// tracerIDContainsSchemaURL is a Probe Const defining whether the tracer key contains schemaURL.
type tracerIDContainsSchemaURL struct{}

// Prior to v1.28 the tracer key did not contain schemaURL. However, in that version a
// change was made to include it.
// https://github.com/open-telemetry/opentelemetry-go/pull/5426/files
var schemaAddedToTracerKeyVer = semver.New(1, 28, 0, "", "")

func (c tracerIDContainsSchemaURL) InjectOption(info *process.Info) (inject.Option, error) {
	ver, ok := info.Modules["go.opentelemetry.io/otel"]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", pkg)
	}

	return inject.WithKeyValue("tracer_id_contains_schemaURL", ver.GreaterThanEqual(schemaAddedToTracerKeyVer)), nil
}

// In v1.32.0 the tracer key was updated to include the scope attributes.
// https://github.com/open-telemetry/opentelemetry-go/pull/5924/files
var scopeAttributesAddedToTracerKeyVer = semver.New(1, 32, 0, "", "")

// tracerIDContainsScopeAttributes is a Probe Const defining whether the tracer key contains scope attributes.
type tracerIDContainsScopeAttributes struct{}

func (c tracerIDContainsScopeAttributes) InjectOption(info *process.Info) (inject.Option, error) {
	ver, ok := info.Modules["go.opentelemetry.io/otel"]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", pkg)
	}

	return inject.WithKeyValue("tracer_id_contains_scope_attributes", ver.GreaterThanEqual(scopeAttributesAddedToTracerKeyVer)), nil
}

type attributeKeyVal struct {
	ValLength uint16
	Vtype     uint8
	Reserved  uint8
	Key       [32]byte
	Value     [128]byte
}

type attributesBuffer struct {
	AttrsKv    [16]attributeKeyVal
	ValidAttrs uint8
}

type status struct {
	Code        uint32
	Description [64]byte
}

type tracerID struct {
	Name      [128]byte
	Version   [32]byte
	SchemaURL [128]byte
}

// event represents a manual span created by the user.
type event struct {
	context.BaseSpanProperties
	SpanName   [64]byte
	Status     status
	Attributes attributesBuffer
	TracerID   tracerID
}

func processFn(e *event) ptrace.ScopeSpans {
	ss := ptrace.NewScopeSpans()

	scope := ss.Scope()
	scope.SetName(unix.ByteSliceToString(e.TracerID.Name[:]))
	scope.SetVersion(unix.ByteSliceToString(e.TracerID.Version[:]))
	ss.SetSchemaUrl(unix.ByteSliceToString(e.TracerID.SchemaURL[:]))

	span := ss.Spans().AppendEmpty()
	span.SetName(unix.ByteSliceToString(e.SpanName[:]))
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(utils.BootOffsetToTimestamp(e.StartTime))
	span.SetEndTimestamp(utils.BootOffsetToTimestamp(e.EndTime))
	span.SetTraceID(pcommon.TraceID(e.SpanContext.TraceID))
	span.SetSpanID(pcommon.SpanID(e.SpanContext.SpanID))
	span.SetFlags(uint32(trace.FlagsSampled))

	if e.ParentSpanContext.SpanID.IsValid() {
		span.SetParentSpanID(pcommon.SpanID(e.ParentSpanContext.SpanID))
	}

	setAttributes(span.Attributes(), e.Attributes)
	setStatus(span.Status(), e.Status)

	return ss
}

func setStatus(dest ptrace.Status, stat status) {
	switch codes.Code(stat.Code) {
	case codes.Unset:
		dest.SetCode(ptrace.StatusCodeUnset)
	case codes.Ok:
		dest.SetCode(ptrace.StatusCodeOk)
	case codes.Error:
		dest.SetCode(ptrace.StatusCodeError)
	}
	dest.SetMessage(unix.ByteSliceToString(stat.Description[:]))
}

func setAttributes(dest pcommon.Map, ab attributesBuffer) {
	for i := 0; i < int(ab.ValidAttrs); i++ {
		akv := ab.AttrsKv[i]
		key := unix.ByteSliceToString(akv.Key[:])
		switch akv.Vtype {
		case uint8(attribute.BOOL):
			dest.PutBool(key, akv.Value[0] != 0)
		case uint8(attribute.INT64):
			v := binary.LittleEndian.Uint64(akv.Value[:8])
			dest.PutInt(key, int64(v)) // nolint: gosec  // Raw value decode.
		case uint8(attribute.FLOAT64):
			v := math.Float64frombits(binary.LittleEndian.Uint64(akv.Value[:8]))
			dest.PutDouble(key, v)
		case uint8(attribute.STRING):
			dest.PutStr(key, unix.ByteSliceToString(akv.Value[:]))
		}
	}
}
