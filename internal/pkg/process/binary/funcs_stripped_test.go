// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package binary

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var relevantFuncs = map[string]any{
	"net/http.HandlerFunc.ServeHTTP":   nil,
	"net/http.serverHandler.ServeHTTP": nil,
	"main.handler":                     nil,
}

// fixture describes a test program to compile.
type fixture struct {
	// dir is the directory of the main package, relative to this package.
	dir string
	// ldflags is passed to go build via -ldflags.
	ldflags string
	// cgo enables cgo for the build, which requires a C compiler.
	cgo bool
	// pie builds a position independent executable.
	pie bool
}

// buildTestBinary compiles the Go program described by f and returns the path
// of the resulting executable.
func buildTestBinary(t *testing.T, f fixture) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found in PATH")
	}
	cgoEnabled := "0"
	if f.cgo {
		if _, err := exec.LookPath("gcc"); err != nil {
			t.Skip("gcc not found in PATH, cannot build cgo binary")
		}
		cgoEnabled = "1"
	}

	out := filepath.Join(t.TempDir(), "prog")
	args := []string{"build", "-buildvcs=false", "-ldflags=" + f.ldflags}
	if f.pie {
		args = append(args, "-buildmode=pie")
	}
	args = append(args, "-o", out, ".")
	//nolint:gosec // Test helper, all arguments are constants or test-owned paths.
	cmd := exec.Command("go", args...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED="+cgoEnabled)

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", output)
	return out
}

func openELF(t *testing.T, path string) *elf.File {
	t.Helper()
	f, err := elf.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// symbolValue returns the value of the named ELF symbol.
func symbolValue(t *testing.T, f *elf.File, name string) uint64 {
	t.Helper()
	syms, err := f.Symbols()
	require.NoError(t, err)
	for _, s := range syms {
		if s.Name == name {
			return s.Value
		}
	}
	t.Fatalf("symbol %q not found", name)
	return 0
}

func TestFindFunctionsStripped(t *testing.T) {
	tests := []struct {
		name string
		// unstripped and stripped describe the two builds to compare. Both
		// need -w so the unstripped variant only differs by the symbol table.
		unstripped, stripped fixture
	}{
		{
			name:       "static",
			unstripped: fixture{dir: "testdata/prog", ldflags: "-w"},
			stripped:   fixture{dir: "testdata/prog", ldflags: "-s -w"},
		},
		{
			// Position independent executables store link-time addresses in
			// the data sections that the loader relocates at run time.
			name:       "static pie",
			unstripped: fixture{dir: "testdata/prog", ldflags: "-w", pie: true},
			stripped:   fixture{dir: "testdata/prog", ldflags: "-s -w", pie: true},
		},
		{
			// External linking places the C runtime before the Go text, so
			// runtime.text is not the start of the .text section.
			name: "cgo external linking",
			unstripped: fixture{
				dir: "testdata/cgoprog", cgo: true, ldflags: "-linkmode=external -w",
			},
			stripped: fixture{
				dir: "testdata/cgoprog", cgo: true, ldflags: "-linkmode=external -s -w",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unstripped := openELF(t, buildTestBinary(t, tt.unstripped))
			stripped := openELF(t, buildTestBinary(t, tt.stripped))
			if stripped.Section(".gopclntab") == nil {
				// Go < 1.26 keeps the pclntab of position independent
				// executables in .data.rel.ro, which FindFunctionsStripped does
				// not support.
				t.Skip("binary has no .gopclntab section")
			}

			// Sanity check the fixtures exercise the intended code paths.
			_, err := stripped.Symbols()
			require.ErrorIs(
				t, err, elf.ErrNoSymbols, "stripped binary must not have a symbol table",
			)
			textSec := unstripped.Section(".text")
			require.NotNil(t, textSec)
			runtimeText := symbolValue(t, unstripped, "runtime.text")
			if tt.unstripped.cgo {
				require.NotEqual(
					t, textSec.Addr, runtimeText,
					"cgo fixture must not have runtime.text at the start of .text",
				)
			}

			want, err := FindFunctionsUnStripped(unstripped, relevantFuncs)
			require.NoError(t, err)
			require.Len(
				t, want, len(relevantFuncs),
				"all relevant functions should be found via the symbol table",
			)

			got, err := FindFunctionsStripped(stripped, relevantFuncs)
			require.NoError(t, err)
			assert.ElementsMatch(
				t, want, got, "stripped lookup must match the symbol table lookup",
			)
		})
	}
}

func TestFindFunctionsUnStrippedNoSymbols(t *testing.T) {
	stripped := openELF(t, buildTestBinary(t, fixture{dir: "testdata/prog", ldflags: "-s -w"}))
	_, err := FindFunctionsUnStripped(stripped, relevantFuncs)
	assert.ErrorIs(t, err, elf.ErrNoSymbols)
}
