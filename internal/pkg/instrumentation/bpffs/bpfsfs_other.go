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

 package bpffs

 import "go.opentelemetry.io/auto/internal/pkg/process"

 // Stubs for non-linux systems

 func PathForTargetApplication(target *process.TargetDetails) string {
 	return ""
 }

 func Mount(target *process.TargetDetails) error {
 	return nil
 }

 func Cleanup(target *process.TargetDetails) error {
 	return nil
 }