#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh
source /test-constants.sh

mock::aws
wait::dbus-ready
mock::kubelet ${CURRENT_VERSION}.0

mock::setup-local-disks

nodeadm init --skip run,install-validation,k8s-authentication-validation --config-source file://config.yaml

assert::file-contains /var/log/setup-local-disks.log 'raid0'
