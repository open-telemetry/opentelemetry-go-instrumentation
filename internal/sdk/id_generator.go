// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var idGeneratorPool = sync.Pool{
	New: func() any {
		gen := &idGenerator{}
		var rngSeed int64
		_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
		gen.rng = rand.New(rand.NewSource(rngSeed))
		return gen
	},
}

func getIDGenerator() *idGenerator {
	return idGeneratorPool.Get().(*idGenerator)
}

func putIDGenerator(idGen *idGenerator) {
	idGeneratorPool.Put(idGen)
}

type idGenerator struct {
	rng *rand.Rand
}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (gen *idGenerator) Generate(traceID trace.TraceID) (trace.TraceID, trace.SpanID) {
	for {
		if traceID.IsValid() {
			// Only overwrite traceID if invalid.
			break
		}
		_, _ = gen.rng.Read(traceID[:])
	}

	var spanID trace.SpanID
	for {
		_, _ = gen.rng.Read(spanID[:])
		if spanID.IsValid() {
			break
		}
	}

	return traceID, spanID
}
