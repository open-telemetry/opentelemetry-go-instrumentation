// Code generated by bpf2go; DO NOT EDIT.
//go:build arm64

package consumer

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfKafkaRequestT struct {
	StartTime     uint64
	EndTime       uint64
	Sc            bpfSpanContext
	Psc           bpfSpanContext
	Topic         [256]int8
	Key           [256]int8
	ConsumerGroup [128]int8
	Offset        int64
	Partition     int64
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
	bpfVariableSpecs
}

// bpfProgramSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfProgramSpecs struct {
	UprobeFetchMessage        *ebpf.ProgramSpec `ebpf:"uprobe_FetchMessage"`
	UprobeFetchMessageReturns *ebpf.ProgramSpec `ebpf:"uprobe_FetchMessage_Returns"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	AllocMap               *ebpf.MapSpec `ebpf:"alloc_map"`
	Events                 *ebpf.MapSpec `ebpf:"events"`
	GoContextToSc          *ebpf.MapSpec `ebpf:"go_context_to_sc"`
	GoroutineToGoContext   *ebpf.MapSpec `ebpf:"goroutine_to_go_context"`
	KafkaEvents            *ebpf.MapSpec `ebpf:"kafka_events"`
	KafkaReaderToConn      *ebpf.MapSpec `ebpf:"kafka_reader_to_conn"`
	KafkaRequestStorageMap *ebpf.MapSpec `ebpf:"kafka_request_storage_map"`
	ProbeActiveSamplerMap  *ebpf.MapSpec `ebpf:"probe_active_sampler_map"`
	SamplersConfigMap      *ebpf.MapSpec `ebpf:"samplers_config_map"`
	SliceArrayBuffMap      *ebpf.MapSpec `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc       *ebpf.MapSpec `ebpf:"tracked_spans_by_sc"`
}

// bpfVariableSpecs contains global variables before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfVariableSpecs struct {
	EndAddr                *ebpf.VariableSpec `ebpf:"end_addr"`
	Hex                    *ebpf.VariableSpec `ebpf:"hex"`
	MessageHeadersPos      *ebpf.VariableSpec `ebpf:"message_headers_pos"`
	MessageKeyPos          *ebpf.VariableSpec `ebpf:"message_key_pos"`
	MessageOffsetPos       *ebpf.VariableSpec `ebpf:"message_offset_pos"`
	MessagePartitionPos    *ebpf.VariableSpec `ebpf:"message_partition_pos"`
	MessageTopicPos        *ebpf.VariableSpec `ebpf:"message_topic_pos"`
	ReaderConfigGroupIdPos *ebpf.VariableSpec `ebpf:"reader_config_group_id_pos"`
	ReaderConfigPos        *ebpf.VariableSpec `ebpf:"reader_config_pos"`
	StartAddr              *ebpf.VariableSpec `ebpf:"start_addr"`
	TotalCpus              *ebpf.VariableSpec `ebpf:"total_cpus"`
}

// bpfObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfObjects struct {
	bpfPrograms
	bpfMaps
	bpfVariables
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
	AllocMap               *ebpf.Map `ebpf:"alloc_map"`
	Events                 *ebpf.Map `ebpf:"events"`
	GoContextToSc          *ebpf.Map `ebpf:"go_context_to_sc"`
	GoroutineToGoContext   *ebpf.Map `ebpf:"goroutine_to_go_context"`
	KafkaEvents            *ebpf.Map `ebpf:"kafka_events"`
	KafkaReaderToConn      *ebpf.Map `ebpf:"kafka_reader_to_conn"`
	KafkaRequestStorageMap *ebpf.Map `ebpf:"kafka_request_storage_map"`
	ProbeActiveSamplerMap  *ebpf.Map `ebpf:"probe_active_sampler_map"`
	SamplersConfigMap      *ebpf.Map `ebpf:"samplers_config_map"`
	SliceArrayBuffMap      *ebpf.Map `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc       *ebpf.Map `ebpf:"tracked_spans_by_sc"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.AllocMap,
		m.Events,
		m.GoContextToSc,
		m.GoroutineToGoContext,
		m.KafkaEvents,
		m.KafkaReaderToConn,
		m.KafkaRequestStorageMap,
		m.ProbeActiveSamplerMap,
		m.SamplersConfigMap,
		m.SliceArrayBuffMap,
		m.TrackedSpansBySc,
	)
}

// bpfVariables contains all global variables after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfVariables struct {
	EndAddr                *ebpf.Variable `ebpf:"end_addr"`
	Hex                    *ebpf.Variable `ebpf:"hex"`
	MessageHeadersPos      *ebpf.Variable `ebpf:"message_headers_pos"`
	MessageKeyPos          *ebpf.Variable `ebpf:"message_key_pos"`
	MessageOffsetPos       *ebpf.Variable `ebpf:"message_offset_pos"`
	MessagePartitionPos    *ebpf.Variable `ebpf:"message_partition_pos"`
	MessageTopicPos        *ebpf.Variable `ebpf:"message_topic_pos"`
	ReaderConfigGroupIdPos *ebpf.Variable `ebpf:"reader_config_group_id_pos"`
	ReaderConfigPos        *ebpf.Variable `ebpf:"reader_config_pos"`
	StartAddr              *ebpf.Variable `ebpf:"start_addr"`
	TotalCpus              *ebpf.Variable `ebpf:"total_cpus"`
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobeFetchMessage        *ebpf.Program `ebpf:"uprobe_FetchMessage"`
	UprobeFetchMessageReturns *ebpf.Program `ebpf:"uprobe_FetchMessage_Returns"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobeFetchMessage,
		p.UprobeFetchMessageReturns,
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
