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

package probe

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

type Const interface {
	InjectOption(td *process.TargetDetails) (inject.Option, error)
}

type consts []Const

func (c consts) structFields() []structfield.ID {
	var out []structfield.ID
	for _, cnst := range c {
		if sfc, ok := cnst.(StructFieldConst); ok {
			out = append(out, sfc.Val)
		}
	}
	return out
}

func (c consts) injectOpts(td *process.TargetDetails) ([]inject.Option, error) {
	var (
		out []inject.Option
		err error
	)
	for _, cnst := range c {
		o, e := cnst.InjectOption(td)
		err = errors.Join(err, e)
		if e == nil {
			out = append(out, o)
		}
	}
	return out, err
}

type StructFieldConst struct {
	Key string
	Val structfield.ID
}

func (c StructFieldConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	ver, ok := td.Libraries[c.Val.ModPath]
	if !ok {
		return nil, fmt.Errorf("unknown module version: %s", c.Val.ModPath)
	}
	return inject.WithOffset(c.Key, c.Val, ver), nil
}

type AllocationConst struct{}

func (c AllocationConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	if td.AllocationDetails == nil {
		return nil, errors.New("no allocation details")
	}
	return inject.WithAllocationDetails(*td.AllocationDetails), nil
}

type RegistersABIConst struct{}

func (c RegistersABIConst) InjectOption(td *process.TargetDetails) (inject.Option, error) {
	return inject.WithRegistersABI(td.IsRegistersABI()), nil
}

type KeyValConst struct {
	Key string
	Val interface{}
}

func (c KeyValConst) InjectOption(*process.TargetDetails) (inject.Option, error) {
	return inject.WithKeyValue(c.Key, c.Val), nil
}
