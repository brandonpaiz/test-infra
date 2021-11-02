#!/bin/bash
# Copyright 2020 gRPC authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -ex

#while getopts "d": flag; do
#  case "${flag}" in
#  d) DRIVER_PORT=${OPTARG} ;;
#  *) echo "Usage $0 -d [driver_port]" >&1
#     exit 1 ;;
#  esac
#done

PROCESSOR_COUNT=$(nproc)
echo "Processor count: ${PROCESSOR_COUNT}"
echo "Driver port: ${DRIVER_PORT}"

BENCHMARK_WORKER_OPTS="-XX:ActiveProcessorCount=${PROCESSOR_COUNT}" \
  timeout --kill-after="${KILL_AFTER}" "${POD_TIMEOUT}" \
  benchmarks/build/install/grpc-benchmarks/bin/benchmark_worker \
  --driver_port="${DRIVER_PORT}"

