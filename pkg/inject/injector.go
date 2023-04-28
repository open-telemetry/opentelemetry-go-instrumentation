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
	"fmt"
	"runtime"

	"github.com/cilium/ebpf"

	"go.opentelemetry.io/auto/pkg/log"
	"go.opentelemetry.io/auto/pkg/process"
)

var (
	//go:embed offset_results.json
	offsetsData string
)

// Injector injects OpenTelemetry instrumentation Go packages.
type Injector struct {
	data              *TrackedOffsets
	isRegAbi          bool
	TotalCPUs         uint32
	AllocationDetails *process.AllocationDetails
}

// New returns an [Injector] configured for the target.
func New(target *process.TargetDetails) (*Injector, error) {
	var offsets TrackedOffsets
	err := json.Unmarshal([]byte(offsetsData), &offsets)
	if err != nil {
		return nil, err
	}

	return &Injector{
		data:              &offsets,
		isRegAbi:          target.IsRegistersABI(),
		TotalCPUs:         uint32(runtime.NumCPU()),
		AllocationDetails: target.AllocationDetails,
	}, nil
}

type loadBpfFunc func() (*ebpf.CollectionSpec, error)

// StructField is the definition of a structure field for which instrumentation
// is injected.
type StructField struct {
	VarName    string
	StructName string
	Field      string
}

// Inject injects instrumentation for the provided library data type.
func (i *Injector) Inject(loadBpf loadBpfFunc, library string, libVersion string, fields []*StructField, initAlloc bool) (*ebpf.CollectionSpec, error) {
	spec, err := loadBpf()
	if err != nil {
		return nil, err
	}

	injectedVars := make(map[string]interface{})

	for _, dm := range fields {
		offset, found := i.getFieldOffset(library, libVersion, dm.StructName, dm.Field)
		if !found {
			log.Logger.V(0).Info("could not find offset", "lib", library, "version", libVersion, "struct", dm.StructName, "field", dm.Field)
		} else {
			injectedVars[dm.VarName] = offset
		}
	}

	if err := i.addCommonInjections(injectedVars, initAlloc); err != nil {
		return nil, fmt.Errorf("adding instrumenter injections: %w", err)
	}
	log.Logger.V(0).Info("Injecting variables", "vars", injectedVars)
	if len(injectedVars) > 0 {
		err = spec.RewriteConstants(injectedVars)
		if err != nil {
			return nil, err
		}
	}

	return spec, nil
}

func (i *Injector) addCommonInjections(varsMap map[string]interface{}, initAlloc bool) error { // nolint:revive  // initAlloc is a control flag.
	varsMap["is_registers_abi"] = i.isRegAbi
	if initAlloc {
		if i.AllocationDetails == nil {
			return fmt.Errorf("couldn't get process allocation details. Try running it from the KeyVal Launcher")
		}
		varsMap["total_cpus"] = i.TotalCPUs
		varsMap["start_addr"] = i.AllocationDetails.StartAddr
		varsMap["end_addr"] = i.AllocationDetails.EndAddr
	}
	return nil
}

func (i *Injector) getFieldOffset(libName string, libVersion string, structName string, fieldName string) (uint64, bool) {
	for _, l := range i.data.Data {
		if l.Name == libName {
			for _, dm := range l.DataMembers {
				if dm.Struct == structName && dm.Field == fieldName {
					for _, o := range dm.Offsets {
						if o.Version == libVersion {
							return o.Offset, true
						}
					}
				}
			}
		}
	}

	return 0, false
}
