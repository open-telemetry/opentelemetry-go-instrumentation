// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inject

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"

	"github.com/cilium/ebpf"
	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/process"
)

var (
	//go:embed offset_results.json
	offsetsData string

	offsets     TrackedOffsets
	errNotFound = errors.New("offset not found")

	nCPU = uint32(runtime.NumCPU())
)

const (
	keyIsRegistersABI = "is_registers_abi"
	keyTotalCPUs      = "total_cpus"
	keyStartAddr      = "start_addr"
	keyEndAddr        = "end_addr"
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
	if len(consts) == 0 {
		return nil
	}
	return spec.RewriteConstants(consts)
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

// WithRegistersABI returns an option that will set the "is_registers_abi" to
// value. This information can be determined from the IsRegistersABI method of
// the TargetDetails in "go.opentelemetry.io/auto/internal/pkg/process".
//
// Commonly this is called like the following:
//
//	WithRegistersABI(target.IsRegistersABI())
func WithRegistersABI(value bool) Option {
	return option{keyIsRegistersABI: value}
}

// WithAllocationDetails returns an option that will set "total_cpus",
// "start_addr", and "end_addr".
func WithAllocationDetails(details process.AllocationDetails) Option {
	return option{
		keyTotalCPUs: nCPU,
		keyStartAddr: details.StartAddr,
		keyEndAddr:   details.EndAddr,
	}
}

// WithKeyValue returns an option that will set key to value.
func WithKeyValue(key string, value interface{}) Option {
	return option{key: value}
}

// WithOffset returns an option that sets key to the offset value of the struct
// field defined by strct and field at the specified version ver.
//
// If the offset value is not known, an error is logged and a no-op Option is
// returned.
func WithOffset(key, strct, field string, ver *version.Version) Option {
	if ver == nil {
		return errOpt{
			err: fmt.Errorf("missing version: %s.%s", strct, field),
		}
	}

	off, ok := offsets.GetOffset(strct, field, ver)
	if !ok {
		return errOpt{
			err: fmt.Errorf("%w: %s.%s (%s)", errNotFound, strct, field, ver),
		}
	}
	return WithKeyValue(key, off)
}
