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

set -e

CILIUM_VERSION=$1

REPOSITORIES="cilium/cilium cilium/operator-generic"
DST_REGISTRY="381492195191.dkr.ecr.us-west-2.amazonaws.com"

if ! command -v oras &> /dev/null; then
    echo "Please install oras"
    exit 1
fi

# We use oras instead of the more typical docker pull/push to make
# sure we mirror all architectures and digets is preserved.
aws ecr get-login-password --region us-west-2 | oras login --username AWS --password-stdin ${DST_REGISTRY}

for repo in $REPOSITORIES; do
	aws ecr create-repository --repository-name ${repo} --region us-west-2 || true

	org=quay.io/${repo}:${CILIUM_VERSION}
	dst=${DST_REGISTRY}/${repo}:${CILIUM_VERSION}
	oras cp ${org} ${dst}
done
