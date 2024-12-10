#!/bin/bash

# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

# launch kafka and wait for it to be ready
/opt/bitnami/scripts/kafka/entrypoint.sh /opt/bitnami/scripts/kafka/run.sh &

while ! kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --topic hc --create --if-not-exists && kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --topic hc --describe; do
  echo "kafka is not available yet. Retrying in 1 second..."
  sleep 1
done
