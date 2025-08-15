#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh
source /test-constants.sh

mock::aws
mock::kubelet ${CURRENT_VERSION}.0
wait::dbus-ready

nodeadm init --skip run,install-validation,k8s-authentication-validation --config-source file://config.yaml

assert::file-contains /etc/eks/kubelet/environment '--v=5 --node-labels=foo=bar,foo2=baz --register-with-taints=foo=bar:NoSchedule"$'
