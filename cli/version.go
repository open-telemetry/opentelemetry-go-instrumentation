// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime"
	"runtime/debug"
	"sync"

	"go.opentelemetry.io/auto"
)

const unknown = "unknown"

var getRevision = sync.OnceValue(func() string {
	rev := unknown

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return rev
	}

	var modified bool
	for _, v := range buildInfo.Settings {
		switch v.Key {
		case "vcs.revision":
			rev = v.Value
		case "vcs.modified":
			modified = v.Value == "true"
		}
	}
	if modified {
		rev += "-dirty"
	}
	return rev
})

type version struct {
	Release  string
	Revision string
	Go       goInfo
}

func newVersion() version {
	v := version{
		Release:  auto.Version(),
		Revision: getRevision(),
		Go:       newGoInfo(),
	}
	return v
}

type goInfo struct {
	Version string
	OS      string
	Arch    string
}

func newGoInfo() goInfo {
	return goInfo{
		Version: runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}
