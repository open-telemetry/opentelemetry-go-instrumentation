#!/bin/bash

# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# launch kafka and wait for it to be ready
/opt/bitnami/scripts/kafka/entrypoint.sh /opt/bitnami/scripts/kafka/run.sh &

while ! kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --topic hc --create --if-not-exists && kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --topic hc --describe; do
  echo "kafka is not available yet. Retrying in 1 second..."
  sleep 1
done

# # Run the Go application
/sample-app/main