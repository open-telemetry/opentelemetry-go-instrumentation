// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package inject provides types and functionality to extract offset
// information from target ELF and inject that data into eBPF probes.
package inject

import (
	"debug/elf"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/cilium/ebpf"

	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

var (
	//go:embed offset_results.json
	offsetsData string

	offsets = structfield.NewIndex()
	// No offset found in the cache.
	errNotFound = errors.New("offset not found")
	// Invalid offset found in the cache. This required field is not supported in the version.
	errInvalid = errors.New("invalid offset for the field in version")
)

const (
	keyTotalCPUs = "total_cpus"
	keyStartAddr = "start_addr"
	keyEndAddr   = "end_addr"
)

func init() {
	err := json.Unmarshal([]byte(offsetsData), &offsets)
	if err != nil {
		// TODO: generate offsets as Go code to avoid this panic.
		panic(err)
	}
}

// Constants injects key-values defined by opts into spec as constant. The keys
// are used as volatile const names and the values are the const values.
//
// If duplicate or colliding Options are passed, the last one passed is used.
func Constants(spec *ebpf.CollectionSpec, opts ...Option) error {
	consts, err := newConsts(opts)
	if err != nil {
		return err
	}

	var missing []string
	for name, val := range consts {
		v, ok := spec.Variables[name]
		if !ok {
			missing = append(missing, name)
			continue
		}

		if !v.Constant() {
			return fmt.Errorf("variable %s is not a constant", name)
		}

		if err := v.Set(val); err != nil {
			return fmt.Errorf("rewriting constant %s: %w", name, err)
		}
	}

	if len(missing) != 0 {
		return fmt.Errorf("rewrite constants: constants are missing from .rodata: %v", missing)
	}

	return nil
}

func newConsts(opts []Option) (map[string]interface{}, error) {
	consts := make(map[string]interface{})
	var err error
	for _, o := range opts {
		err = errors.Join(err, o.apply(consts))
	}
	return consts, err
}

// Option configures key-values to be injected into an [ebpf.CollectionSpec].
type Option interface {
	apply(map[string]interface{}) error
}

type option map[string]interface{}

func (o option) apply(m map[string]interface{}) error {
	for key, val := range o {
		m[key] = val
	}
	return nil
}

type errOpt struct {
	err error
}

func (o errOpt) apply(map[string]interface{}) error {
	return o.err
}

// WithAllocation returns an option that will set "total_cpus", "start_addr",
// and "end_addr".
func WithAllocation(alloc process.Allocation) Option {
	return option{
		keyTotalCPUs: alloc.NumCPU,
		keyStartAddr: alloc.StartAddr,
		keyEndAddr:   alloc.EndAddr,
	}
}

// WithKeyValue returns an option that will set key to value.
func WithKeyValue(key string, value interface{}) Option {
	return option{key: value}
}

// WithOffset returns an option that sets key to the offset value of the struct
// field defined by id at the specified version ver.
//
// If the offset value is not known, an error is returned when the returned
// Option is used.
func WithOffset(key string, id structfield.ID, ver *semver.Version) Option {
	if ver == nil {
		return errOpt{
			err: fmt.Errorf("missing version: %s", id),
		}
	}

	off, ok := offsets.GetOffset(id, ver)
	if !ok {
		return errOpt{
			err: fmt.Errorf("%w: %s (%s)", errNotFound, id, ver),
		}
	}
	if !off.Valid {
		return errOpt{
			err: fmt.Errorf("%w: %s (%s)", errInvalid, id, ver),
		}
	}
	return WithKeyValue(key, off.Offset)
}

func FindOffset(id structfield.ID, info *process.Info) (structfield.OffsetKey, error) {
	elfF, err := elf.Open(info.ID.ExePath())
	if err != nil {
		return structfield.OffsetKey{}, err
	}
	defer elfF.Close()

	data, err := elfF.DWARF()
	if err != nil {
		return structfield.OffsetKey{}, err
	}

	v, err := process.DWARF{Reader: data.Reader()}.GoStructField(id)
	if err != nil {
		return structfield.OffsetKey{}, err
	}
	if v < 0 {
		return structfield.OffsetKey{}, fmt.Errorf("invalid offset: %d", v)
	}
	return structfield.OffsetKey{Offset: uint64(v), Valid: true}, err // nolint: gosec  // Bounded.
}

func GetOffset(id structfield.ID, ver *semver.Version) (structfield.OffsetKey, bool) {
	return offsets.GetOffset(id, ver)
}

func GetLatestOffset(id structfield.ID) (structfield.OffsetKey, *semver.Version) {
	return offsets.GetLatestOffset(id)
}
