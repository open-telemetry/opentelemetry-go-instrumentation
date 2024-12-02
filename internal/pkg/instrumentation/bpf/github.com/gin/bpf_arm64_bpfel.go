// Code generated by bpf2go; DO NOT EDIT.
//go:build arm64

package gin

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfSliceArrayBuff struct{ Buff [1024]uint8 }

type bpfSpanContext struct {
	TraceID    [16]uint8
	SpanID     [8]uint8
	TraceFlags uint8
	Padding    [7]uint8
}

type bpfUprobeDataT struct {
	Req struct {
		StartTime   uint64
		EndTime     uint64
		Sc          bpfSpanContext
		Psc         bpfSpanContext
		Method      [8]int8
		Path        [128]int8
		PathPattern [128]int8
	}
	GinCtxPtr uint64
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
	UprobeGinEngineHandleHTTPRequest        *ebpf.ProgramSpec `ebpf:"uprobe_GinEngine_handleHTTPRequest"`
	UprobeGinEngineHandleHTTPRequestReturns *ebpf.ProgramSpec `ebpf:"uprobe_GinEngine_handleHTTPRequest_Returns"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	AllocMap              *ebpf.MapSpec `ebpf:"alloc_map"`
	Events                *ebpf.MapSpec `ebpf:"events"`
	GinUprobeStorageMap   *ebpf.MapSpec `ebpf:"gin_uprobe_storage_map"`
	GoContextToSc         *ebpf.MapSpec `ebpf:"go_context_to_sc"`
	HttpEvents            *ebpf.MapSpec `ebpf:"http_events"`
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
	AllocMap              *ebpf.Map `ebpf:"alloc_map"`
	Events                *ebpf.Map `ebpf:"events"`
	GinUprobeStorageMap   *ebpf.Map `ebpf:"gin_uprobe_storage_map"`
	GoContextToSc         *ebpf.Map `ebpf:"go_context_to_sc"`
	HttpEvents            *ebpf.Map `ebpf:"http_events"`
	ProbeActiveSamplerMap *ebpf.Map `ebpf:"probe_active_sampler_map"`
	SamplersConfigMap     *ebpf.Map `ebpf:"samplers_config_map"`
	SliceArrayBuffMap     *ebpf.Map `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc      *ebpf.Map `ebpf:"tracked_spans_by_sc"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.AllocMap,
		m.Events,
		m.GinUprobeStorageMap,
		m.GoContextToSc,
		m.HttpEvents,
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
	UprobeGinEngineHandleHTTPRequest        *ebpf.Program `ebpf:"uprobe_GinEngine_handleHTTPRequest"`
	UprobeGinEngineHandleHTTPRequestReturns *ebpf.Program `ebpf:"uprobe_GinEngine_handleHTTPRequest_Returns"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeGinEngineHandleHTTPRequest,
		p.UprobeGinEngineHandleHTTPRequestReturns,
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
//go:embed bpf_arm64_bpfel.o
var _BpfBytes []byte
