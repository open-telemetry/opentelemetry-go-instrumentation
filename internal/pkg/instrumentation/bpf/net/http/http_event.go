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

package http

import (
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
