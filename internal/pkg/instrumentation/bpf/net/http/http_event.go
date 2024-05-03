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

// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"golang.org/x/sys/unix"
)

func ServerAddressPortAttributes(host []byte) (addr attribute.KeyValue, port attribute.KeyValue) {
	var portString string
	var e error
	hostString := unix.ByteSliceToString(host)

	if strings.Contains(hostString, ":") {
		if hostString, portString, e = net.SplitHostPort(hostString); e == nil {
			if portI, err := strconv.Atoi(portString); err == nil {
				port = semconv.ServerPort(portI)
			}
		}
	}

	if hostString != "" {
		addr = semconv.ServerAddress(hostString)
	}
	return
}

func NetPeerAddressPortAttributes(host []byte) (addr attribute.KeyValue, port attribute.KeyValue) {
	var portString string
	var e error
	hostString := unix.ByteSliceToString(host)

	if strings.Contains(hostString, ":") {
		if hostString, portString, e = net.SplitHostPort(hostString); e == nil {
			if portI, err := strconv.Atoi(portString); err == nil {
				port = semconv.NetworkPeerPort(portI)
			}
		}
	}

	if hostString != "" {
		addr = semconv.NetworkPeerAddress(hostString)
	}
	return
}

var (
	// ErrEmptyPattern is returned when the input pattern is empty.
	ErrEmptyPattern = errors.New("empty pattern")
	// ErrMissingPathOrHost is returned when the input pattern is missing path or host.
	ErrMissingPathOrHost = errors.New("missing path or host")
)

// The string's syntax is
//
//	[METHOD] [HOST]/[PATH]
//
// https://cs.opensource.google/go/go/+/master:src/net/http/pattern.go;l=84;drc=b47f2febea5c570fef4a5c27a46473f511fbdaa3?q=PATTERN%20STRUCT&ss=go%2Fgo
func ParsePattern(s string) (path string, err error) {
	if len(s) == 0 {
		return "", ErrEmptyPattern
	}

	method, rest, found := s, "", false
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		method, rest, found = s[:i], strings.TrimLeft(s[i+1:], " \t"), true
	}
	if !found {
		rest = method
	}

	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return "", ErrMissingPathOrHost
	}
	path = rest[i:]
	err = nil
	return
}
