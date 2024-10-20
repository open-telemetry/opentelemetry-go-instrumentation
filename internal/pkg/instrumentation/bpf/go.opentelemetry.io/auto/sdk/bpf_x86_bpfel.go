// Code generated by bpf2go; DO NOT EDIT.
//go:build 386 || amd64

package sdk

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfOtelSpanT struct {
	Sc  bpfSpanContext
	Psc bpfSpanContext
}

type bpfSliceArrayBuff struct{ Buff [1024]uint8 }

type bpfSpanContext struct {
	TraceID    [16]uint8
	SpanID     [8]uint8
	TraceFlags uint8
	Padding    [7]uint8
}

// loadBpf returns the embedded CollectionSpec for bpf.
func loadBpf() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BpfBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load bpf: %w", err)
	}

	return spec, err
}

// loadBpfObjects loads bpf and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*bpfObjects
//	*bpfPrograms
//	*bpfMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadBpfObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadBpf()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// bpfSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfSpecs struct {
	bpfProgramSpecs
	bpfMapSpecs
}

// bpfSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfProgramSpecs struct {
	UprobeSpanEnded   *ebpf.ProgramSpec `ebpf:"uprobe_Span_ended"`
	UprobeTracerStart *ebpf.ProgramSpec `ebpf:"uprobe_Tracer_start"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	ActiveSpansBySpanPtr  *ebpf.MapSpec `ebpf:"active_spans_by_span_ptr"`
	AllocMap              *ebpf.MapSpec `ebpf:"alloc_map"`
	Events                *ebpf.MapSpec `ebpf:"events"`
	GoContextToSc         *ebpf.MapSpec `ebpf:"go_context_to_sc"`
	NewEvent              *ebpf.MapSpec `ebpf:"new_event"`
	ProbeActiveSamplerMap *ebpf.MapSpec `ebpf:"probe_active_sampler_map"`
	SamplersConfigMap     *ebpf.MapSpec `ebpf:"samplers_config_map"`
	SliceArrayBuffMap     *ebpf.MapSpec `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc      *ebpf.MapSpec `ebpf:"tracked_spans_by_sc"`
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
}

func (o *bpfObjects) Close() error {
	return _BpfClose(
		&o.bpfPrograms,
		&o.bpfMaps,
	)
}

// bpfMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfMaps struct {
	ActiveSpansBySpanPtr  *ebpf.Map `ebpf:"active_spans_by_span_ptr"`
	AllocMap              *ebpf.Map `ebpf:"alloc_map"`
	Events                *ebpf.Map `ebpf:"events"`
	GoContextToSc         *ebpf.Map `ebpf:"go_context_to_sc"`
	NewEvent              *ebpf.Map `ebpf:"new_event"`
	ProbeActiveSamplerMap *ebpf.Map `ebpf:"probe_active_sampler_map"`
	SamplersConfigMap     *ebpf.Map `ebpf:"samplers_config_map"`
	SliceArrayBuffMap     *ebpf.Map `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc      *ebpf.Map `ebpf:"tracked_spans_by_sc"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.ActiveSpansBySpanPtr,
		m.AllocMap,
		m.Events,
		m.GoContextToSc,
		m.NewEvent,
		m.ProbeActiveSamplerMap,
		m.SamplersConfigMap,
		m.SliceArrayBuffMap,
		m.TrackedSpansBySc,
	)
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobeSpanEnded   *ebpf.Program `ebpf:"uprobe_Span_ended"`
	UprobeTracerStart *ebpf.Program `ebpf:"uprobe_Tracer_start"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeSpanEnded,
		p.UprobeTracerStart,
	)
}

func _BpfClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_x86_bpfel.o
var _BpfBytes []byte
