#!/usr/bin/env bash
# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Short-circuit if script has already been sourced
[[ $(type -t build::common::loaded) == function ]] && return 0

function build::common::echo_and_run() {
    >&2 echo "($(pwd)) \$ $*"
    "$@"
}

function fail() {
  echo $1 >&2
  exit 1
}

function retry() {
    local n=1
    local max=40
    local delay=10
    while true; do
        "$@" && break || {
            if [[ $n -lt $max ]]; then
                ((n++))
                >&2 echo "Command failed. Attempt $n/$max:"
                sleep $delay;
            else
                fail "The command has failed after $n attempts."
            fi
        }
    done
}

# Marker function to indicate script has been fully sourced
function build::common::loaded() {
  return 0
}
