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

package inspect

import "github.com/hashicorp/go-version"

type Manifest struct {
	// Application is the application to extract binary data from.
	Application Application
	// StructFields are struct fields the application should contain that need
	// offsets to be found.
	StructFields []StructField
}

// Application is the information about a template application that needs to be
// inspected for binary data.
type Application struct {
	// Renderer renders the application.
	Renderer Renderer
	// Versions are the application versions to be inspected. They will be
	// passed to the Renderer as the ".Version" field.
	//
	// If this is nil, the GoVerions will also be used as the application
	// versions that are passed to the template.
	Versions []*version.Version
	// GoVerions are the versions of Go to build the application with.
	//
	// If this is nil, the latest version of Go will be used.
	GoVerions []*version.Version
}
