// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"log/slog"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/auto/internal/pkg/inject"
	"go.opentelemetry.io/auto/internal/pkg/process"
	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// transportPkg is the package declaring the structs a loopyWriter handles.
const transportPkg = "google.golang.org/grpc/internal/transport"

func TestHeaderOffsetStructSelection(t *testing.T) {
	testCases := []struct {
		name string
		// want is the struct expected to declare the fields at this version.
		want string
	}{
		{name: "1.40.0", want: "headerFrame"},
		{name: "1.81.1", want: "headerFrame"},
		{name: "1.82.0", want: "headerFrame"},
		{name: "1.82.0-dev", want: "headerFrame"},
		{name: "1.82.1", want: "clientHeaders"},
		{name: "1.83.0", want: "clientHeaders"},
		// Snapshots taken before the split reached them.
		{name: "1.83.0-dev", want: "headerFrame"},
		{name: "1.84.0-dev", want: "headerFrame"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ver, err := semver.NewVersion(tc.name)
			require.NoError(t, err)

			for _, field := range []string{"hf", "streamID"} {
				for _, strct := range []string{"headerFrame", "clientHeaders"} {
					id := structfield.NewID(pkg, transportPkg, strct, field)

					off, ok := inject.GetOffset(id, ver)
					assert.Equal(
						t,
						strct == tc.want,
						ok && off.Valid,
						"%s.%s at %s",
						strct,
						field,
						tc.name,
					)
				}
			}
		})
	}
}

func TestHeaderOffsetConstResolves(t *testing.T) {
	versions := []string{
		"1.40.0",
		"1.82.0",
		"1.82.1",
		"1.83.0",
		"1.83.0-dev",
		"1.84.0-dev",
	}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			ver, err := semver.NewVersion(version)
			require.NoError(t, err)

			info := &process.Info{Modules: map[string]*semver.Version{pkg: ver}}

			for _, field := range []string{"hf", "streamID"} {
				c := headerOffsetConst{
					Key: "headerFrame_" + field + "_pos",
					IDs: []structfield.ID{
						structfield.NewID(pkg, transportPkg, "headerFrame", field),
						structfield.NewID(pkg, transportPkg, "clientHeaders", field),
					},
				}

				opt, err := c.InjectOption(info)
				assert.NoError(t, err, c.Key)
				assert.NotNil(t, opt, c.Key)
			}
		})
	}
}

func TestManifestIncludesHeaderStructFields(t *testing.T) {
	fields := New(slog.Default(), "").Manifest().StructFields

	for _, field := range []string{"hf", "streamID"} {
		for _, strct := range []string{"headerFrame", "clientHeaders"} {
			assert.Contains(
				t,
				fields,
				structfield.NewID(pkg, transportPkg, strct, field),
			)
		}
	}
}

func TestHeaderOffsetConstUnknownModule(t *testing.T) {
	c := headerOffsetConst{
		Key: "headerFrame_hf_pos",
		IDs: []structfield.ID{
			structfield.NewID(pkg, transportPkg, "headerFrame", "hf"),
		},
	}

	_, err := c.InjectOption(&process.Info{Modules: make(map[string]*semver.Version)})
	assert.ErrorContains(t, err, pkg)
}

func TestHeaderOffsetConstNoValidOffset(t *testing.T) {
	// 1.13.0 predates both structs, so no offset is recorded for either. The
	// binary of the process is analyzed as a fallback, which fails for this
	// process that does not exist.
	ver, err := semver.NewVersion("1.13.0")
	require.NoError(t, err)

	c := headerOffsetConst{
		Key: "headerFrame_hf_pos",
		IDs: []structfield.ID{
			structfield.NewID(pkg, transportPkg, "headerFrame", "hf"),
			structfield.NewID(pkg, transportPkg, "clientHeaders", "hf"),
		},
	}

	opt, err := c.InjectOption(&process.Info{
		Modules: map[string]*semver.Version{pkg: ver},
	})
	assert.ErrorContains(t, err, c.Key)
	assert.Nil(t, opt)
}
