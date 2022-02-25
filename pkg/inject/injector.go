package inject

import (
	_ "embed"
	"encoding/json"
	"github.com/cilium/ebpf"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
)

var (
	//go:embed offset_results.json
	offsetsData string
)

type Injector struct {
	data *TrackedOffsets
}

func New() (*Injector, error) {
	var offsets TrackedOffsets
	err := json.Unmarshal([]byte(offsetsData), &offsets)
	if err != nil {
		return nil, err
	}

	return &Injector{
		data: &offsets,
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

	offsets := make(map[string]interface{})

	for _, dm := range fields {
		offset, found := i.getFieldOffset(library, libVersion, dm.StructName, dm.Field)
		if !found {
			log.Logger.V(0).Info("could not find offset", "lib", library, "version", libVersion, "struct", dm.StructName, "field", dm.Field)
		} else {
			offsets[dm.VarName] = offset
		}
	}

	log.Logger.V(0).Info("Injecting offsets", "offsets", offsets)
	if len(offsets) > 0 {
		err = spec.RewriteConstants(offsets)
		if err != nil {
			return nil, err
		}
	}

	return spec, nil
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
