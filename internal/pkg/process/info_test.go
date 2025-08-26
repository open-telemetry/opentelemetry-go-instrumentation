// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
	"log/slog"
	"sync"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoVer(t *testing.T) {
	tests := []struct {
		in   string
		want *semver.Version
	}{
		{
			in:   "go1.20",
			want: semver.MustParse("1.20"),
		},
		{
			in:   "go1.20.1",
			want: semver.MustParse("1.20.1"),
		},
		{
			in:   "go1.20.1 X:nocoverageredesign",
			want: semver.MustParse("1.20.1"),
		},
		{
			in:   "go1.18+",
			want: semver.MustParse("1.18"),
		},
		{
			in:   "devel +8e496f1 Thu Nov 5 15:41:05 2015 +0000",
			want: semver.New(0, 0, 0, "", "8e496f1"),
		},
		{
			in:   "v0.0.0-20191109021931-daa7c04131f5+01",
			want: semver.MustParse("v0.0.0-20191109021931-daa7c04131f5+01"),
		},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got, err := goVer(test.in)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestInfoAlloc(t *testing.T) {
	setup := func(t *testing.T, err error) {
		t.Helper()

		orig := allocateFn

		a := new(Allocation)
		allocateFn = func(*slog.Logger, ID) (*Allocation, error) {
			a.StartAddr++
			return a, err
		}
		t.Cleanup(func() { allocateFn = orig })
	}

	logger := slog.Default()
	const goroutines = 10

	t.Run("ErrorReturn", func(t *testing.T) {
		setup(t, assert.AnError)

		i := new(Info)
		var wg sync.WaitGroup
		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := i.Alloc(logger)
				assert.ErrorIs(t, err, assert.AnError)
			}()
		}
		wg.Wait()

		a, err := i.Alloc(logger)
		assert.ErrorIs(t, err, assert.AnError) //nolint:testifylint // Continue on failure.
		assert.Equal(t, uint64(goroutines+1), a.StartAddr, "expected increment per error response")
	})

	t.Run("SuccessCached", func(t *testing.T) {
		setup(t, nil)

		i := new(Info)
		var wg sync.WaitGroup
		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := i.Alloc(logger)
				assert.NoError(t, err)
			}()
		}
		wg.Wait()

		a, err := i.Alloc(logger)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), a.StartAddr, "allocate not called once")
	})
}
