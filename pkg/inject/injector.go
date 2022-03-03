package inject

import (
	_ "embed"
	"encoding/json"
	"github.com/cilium/ebpf"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/process"
)

var (
	//go:embed offset_results.json
	offsetsData string
)

type Injector struct {
	data     *TrackedOffsets
	isRegAbi bool
}

func New(target *process.TargetDetails) (*Injector, error) {
	var offsets TrackedOffsets
	err := json.Unmarshal([]byte(offsetsData), &offsets)
	if err != nil {
		return nil, err
	}

	return &Injector{
		data:     &offsets,
		isRegAbi: target.IsRegistersABI(),
	}, nil
}

type loadBpfFunc func() (*ebpf.CollectionSpec, error)

type InjectStructField struct {
	VarName    string
	StructName string
	Field      string
}

func (i *Injector) Inject(loadBpf loadBpfFunc, library string, libVersion string, fields []*InjectStructField) (*ebpf.CollectionSpec, error) {
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

	i.addCommonInjections(injectedVars)
	log.Logger.V(0).Info("Injecting variables", "vars", injectedVars)
	if len(injectedVars) > 0 {
		err = spec.RewriteConstants(injectedVars)
		if err != nil {
			return nil, err
		}
	}

	return spec, nil
}

func (i *Injector) addCommonInjections(varsMap map[string]interface{}) {
	varsMap["is_registers_abi"] = i.isRegAbi
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
