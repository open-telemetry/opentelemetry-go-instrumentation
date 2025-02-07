// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"errors"

	"github.com/Masterminds/semver/v3"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// Manifest contains all information that needs to be inspected for an
// application.
type Manifest struct {
	// Application is the application to extract binary data from.
	Application Application
	// StructFields are struct fields the application should contain that need
	// offsets to be found.
	StructFields []structfield.ID
}

func (m Manifest) validate() error {
	if m.Application.GoVerions == nil && m.Application.Versions == nil {
		return errors.New("missing version: a Go or application version is required")
	}
	return nil
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
	Versions []*semver.Version
	// GoVerions are the versions of Go to build the application with.
	//
	// If this is nil, the latest version of Go will be used.
	GoVerions []*semver.Version
}
