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

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

CLUSTER_NAME="${1?Please specifiy the Cluster Name}"
REGION="${2?Please specify the AWS region}"
KUBERNETES_VERSION="${3?Please specify the Kubernetes version}"
CNI="${4?Please specify the cni}"
NODEADM_AMD_URL="${5?Please specify the nodeadm amd url}"
NODEADM_ARM_URL="${6?Please specify the nodeadm arm url}"

CONFIG_DIR="$REPO_ROOT/e2e-config"
BIN_DIR="$REPO_ROOT/_bin"

mkdir -p $CONFIG_DIR

cat <<EOF > $CONFIG_DIR/e2e-setup-spec.yaml
spec:
  clusterName: $CLUSTER_NAME
  clusterRegion: $REGION
  kubernetesVersions:
    - $KUBERNETES_VERSION
  cni: $CNI
  clusterNetwork:
    vpcCidr: 10.0.0.0/16
    publicSubnetCidr: 10.0.10.0/24
    privateSubnetCidr: 10.0.20.0/24
  hybridNetwork:
    vpcCidr: 10.1.0.0/16
    publicSubnetCidr: 10.1.1.0/24
    privateSubnetCidr: 10.1.2.0/24
    podCidr: 10.2.0.0/16
EOF

RESOURCES_YAML=$CONFIG_DIR/$CLUSTER_NAME-resources.yaml
$BIN_DIR/e2e-test-runner setup -s $CONFIG_DIR/e2e-setup-spec.yaml

trap "$BIN_DIR/e2e-test-runner cleanup -f $RESOURCES_YAML" EXIT
mv /tmp/setup-resources-output.yaml $RESOURCES_YAML

VPC_ID="$(yq -r '.status.hybridVpcID' $RESOURCES_YAML)"

cat <<EOF > $CONFIG_DIR/e2e-param.yaml
clusterName: "$CLUSTER_NAME-${KUBERNETES_VERSION/./-}"
clusterRegion: "$REGION"
hybridVpcID: "$VPC_ID"
nodeadmUrl: "$NODEADM_AMD_URL"
EOF

$BIN_DIR/ginkgo -v -tags=e2e --label-filter='ssm' $BIN_DIR/e2e.test -- -filepath=$CONFIG_DIR/e2e-param.yaml
