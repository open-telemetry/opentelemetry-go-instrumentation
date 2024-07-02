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
 
 //go:build !linux

 package ptrace

 import "github.com/go-logr/logr"

 // Stubs for non-linux systems

 type TracedProgram struct {}

 func NewTracedProgram(pid int, logger logr.Logger) (*TracedProgram, error) {
 	return nil, nil
 }

 func (p *TracedProgram) Detach() error {
 	return nil
 }

 func (p *TracedProgram) SetMemLockInfinity() error {
 	return nil
 }

 func (p *TracedProgram) Mmap(length uint64, fd uint64) (uint64, error) {
 	return 0, nil
 }

 func (p *TracedProgram) Madvise(addr uint64, length uint64) error {
 	return nil
 }

 func (p *TracedProgram) Mlock(addr uint64, length uint64) error {
 	return nil
 }
 