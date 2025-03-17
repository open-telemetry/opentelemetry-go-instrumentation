// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package process

import (
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
