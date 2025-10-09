#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh
source /test-constants.sh

mock::aws
wait::dbus-ready

declare INITIAL_VERSION=$DEFAULT_INITIAL_VERSION
declare TARGET_VERSION=$CURRENT_VERSION

# remove previously installed containerd to ensure nodeadm tracks containerd source as distro
dnf remove -y containerd
nodeadm install $INITIAL_VERSION --credential-provider ssm
mock::ssm
nodeadm init --skip run,preprocess,node-ip-validation,k8s-authentication-validation --config-source file://config.yaml

# Test 1: Start with containerd 1.x, skip upgrade should stay 1.x
install-containerd-version "1.7.27-1.amzn2023.0.1"
assert-containerd-major-version "1"
nodeadm upgrade $TARGET_VERSION --skip containerd-major-version-upgrade,run,preprocess,pod-validation,node-validation,init-validation,node-ip-validation,k8s-authentication-validation --config-source file://config.yaml
assert-containerd-major-version "1"

# Test 2: Start with containerd 1.x, normal upgrade should go to 2.x
install-containerd-version "1.7.27-1.amzn2023.0.1"
assert-containerd-major-version "1"
nodeadm upgrade $TARGET_VERSION --skip run,preprocess,pod-validation,node-validation,init-validation,node-ip-validation,k8s-authentication-validation --config-source file://config.yaml
assert-containerd-major-version "2"

# Test 3: Start with containerd 2.x, skip upgrade should stay 2.x
install-containerd-version "2.0.5-1.amzn2023.0.1"
assert-containerd-major-version "2"
nodeadm upgrade $TARGET_VERSION --skip containerd-major-version-upgrade,run,preprocess,pod-validation,node-validation,init-validation,node-ip-validation,k8s-authentication-validation --config-source file://config.yaml
assert-containerd-major-version "2"

# Test 4: Start with containerd 2.x, normal upgrade should stay 2.x
install-containerd-version "2.0.5-1.amzn2023.0.1"
assert-containerd-major-version "2"
nodeadm upgrade $TARGET_VERSION --skip run,preprocess,pod-validation,node-validation,init-validation,node-ip-validation,k8s-authentication-validation --config-source file://config.yaml
assert-containerd-major-version "2"
