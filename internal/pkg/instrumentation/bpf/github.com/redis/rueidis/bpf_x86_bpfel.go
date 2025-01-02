// Code generated by bpf2go; DO NOT EDIT.
//go:build 386 || amd64

package rueidis

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type bpfRueidisCompletedCommandT struct {
	StartTime     uint64
	EndTime       uint64
	Sc            bpfSpanContext
	Psc           bpfSpanContext
	OperationName [20]int8
	LocalAddr     struct {
		Ip   [16]uint8
		Port uint32
	}
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
	UprobePipeDo        *ebpf.ProgramSpec `ebpf:"uprobe_pipe_Do"`
	UprobePipeDoReturns *ebpf.ProgramSpec `ebpf:"uprobe_pipe_Do_Returns"`
}

// bpfMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfMapSpecs struct {
	AllocMap              *ebpf.MapSpec `ebpf:"alloc_map"`
	Events                *ebpf.MapSpec `ebpf:"events"`
	GoContextToSc         *ebpf.MapSpec `ebpf:"go_context_to_sc"`
	ProbeActiveSamplerMap *ebpf.MapSpec `ebpf:"probe_active_sampler_map"`
	RedisCompletedEvents  *ebpf.MapSpec `ebpf:"redis_completed_events"`
	SamplersConfigMap     *ebpf.MapSpec `ebpf:"samplers_config_map"`
	SliceArrayBuffMap     *ebpf.MapSpec `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc      *ebpf.MapSpec `ebpf:"tracked_spans_by_sc"`
}

// bpfVariableSpecs contains global variables before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type bpfVariableSpecs struct {
	TCPAddrIP_offset  *ebpf.VariableSpec `ebpf:"TCPAddr_IP_offset"`
	TCPAddrPortOffset *ebpf.VariableSpec `ebpf:"TCPAddr_Port_offset"`
	CompletedCsPos    *ebpf.VariableSpec `ebpf:"completed_cs_pos"`
	ConnFdPos         *ebpf.VariableSpec `ebpf:"conn_fd_pos"`
	CsS_pos           *ebpf.VariableSpec `ebpf:"cs_s_pos"`
	EndAddr           *ebpf.VariableSpec `ebpf:"end_addr"`
	FdRaddrPos        *ebpf.VariableSpec `ebpf:"fd_raddr_pos"`
	Hex               *ebpf.VariableSpec `ebpf:"hex"`
	IsRegistersAbi    *ebpf.VariableSpec `ebpf:"is_registers_abi"`
	MaxOprationLength *ebpf.VariableSpec `ebpf:"max_opration_length"`
	PipeConnPos       *ebpf.VariableSpec `ebpf:"pipe_conn_pos"`
	ResultErrorPos    *ebpf.VariableSpec `ebpf:"result_error_pos"`
	StartAddr         *ebpf.VariableSpec `ebpf:"start_addr"`
	TcpAddrIpPos      *ebpf.VariableSpec `ebpf:"tcp_addr_ip_pos"`
	TcpAddrPortPos    *ebpf.VariableSpec `ebpf:"tcp_addr_port_pos"`
	TcpConnConnPos    *ebpf.VariableSpec `ebpf:"tcp_conn_conn_pos"`
	TotalCpus         *ebpf.VariableSpec `ebpf:"total_cpus"`
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
	AllocMap              *ebpf.Map `ebpf:"alloc_map"`
	Events                *ebpf.Map `ebpf:"events"`
	GoContextToSc         *ebpf.Map `ebpf:"go_context_to_sc"`
	ProbeActiveSamplerMap *ebpf.Map `ebpf:"probe_active_sampler_map"`
	RedisCompletedEvents  *ebpf.Map `ebpf:"redis_completed_events"`
	SamplersConfigMap     *ebpf.Map `ebpf:"samplers_config_map"`
	SliceArrayBuffMap     *ebpf.Map `ebpf:"slice_array_buff_map"`
	TrackedSpansBySc      *ebpf.Map `ebpf:"tracked_spans_by_sc"`
}

func (m *bpfMaps) Close() error {
	return _BpfClose(
		m.AllocMap,
		m.Events,
		m.GoContextToSc,
		m.ProbeActiveSamplerMap,
		m.RedisCompletedEvents,
		m.SamplersConfigMap,
		m.SliceArrayBuffMap,
		m.TrackedSpansBySc,
	)
}

// bpfVariables contains all global variables after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfVariables struct {
	TCPAddrIP_offset  *ebpf.Variable `ebpf:"TCPAddr_IP_offset"`
	TCPAddrPortOffset *ebpf.Variable `ebpf:"TCPAddr_Port_offset"`
	CompletedCsPos    *ebpf.Variable `ebpf:"completed_cs_pos"`
	ConnFdPos         *ebpf.Variable `ebpf:"conn_fd_pos"`
	CsS_pos           *ebpf.Variable `ebpf:"cs_s_pos"`
	EndAddr           *ebpf.Variable `ebpf:"end_addr"`
	FdRaddrPos        *ebpf.Variable `ebpf:"fd_raddr_pos"`
	Hex               *ebpf.Variable `ebpf:"hex"`
	IsRegistersAbi    *ebpf.Variable `ebpf:"is_registers_abi"`
	MaxOprationLength *ebpf.Variable `ebpf:"max_opration_length"`
	PipeConnPos       *ebpf.Variable `ebpf:"pipe_conn_pos"`
	ResultErrorPos    *ebpf.Variable `ebpf:"result_error_pos"`
	StartAddr         *ebpf.Variable `ebpf:"start_addr"`
	TcpAddrIpPos      *ebpf.Variable `ebpf:"tcp_addr_ip_pos"`
	TcpAddrPortPos    *ebpf.Variable `ebpf:"tcp_addr_port_pos"`
	TcpConnConnPos    *ebpf.Variable `ebpf:"tcp_conn_conn_pos"`
	TotalCpus         *ebpf.Variable `ebpf:"total_cpus"`
}

// bpfPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadBpfObjects or ebpf.CollectionSpec.LoadAndAssign.
type bpfPrograms struct {
	UprobePipeDo        *ebpf.Program `ebpf:"uprobe_pipe_Do"`
	UprobePipeDoReturns *ebpf.Program `ebpf:"uprobe_pipe_Do_Returns"`
}

func (p *bpfPrograms) Close() error {
	return _BpfClose(
		p.UprobePipeDo,
		p.UprobePipeDoReturns,
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
