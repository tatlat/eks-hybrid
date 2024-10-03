#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::aws
wait::dbus-ready

declare INITIAL_VERSION=1.26
declare TARGET_VERSION=1.30

# remove previously installed containerd to test installation via nodeadm
dnf remove -y containerd

# Test nodeadm upgrade with iam as credential provider
# initial: version 1.26
# target: version 1.30
nodeadm install $INITIAL_VERSION --credential-provider iam-ra
assert::path-exists /usr/bin/containerd
assert::path-exists /usr/sbin/iptables
assert::path-exists /usr/bin/kubelet
assert::path-exists /usr/local/bin/kubectl
VERSION_INFO=$(/usr/local/bin/kubectl version || true)
assert::is-substring "$VERSION_INFO" "v$INITIAL_VERSION"
assert::path-exists /opt/cni/bin/
assert::path-exists /etc/eks/image-credential-provider/ecr-credential-provider
assert::path-exists /usr/local/bin/aws-iam-authenticator
assert::path-exists /usr/local/bin/aws_signing_helper
assert::files-equal /opt/aws/nodeadm-tracker expected-nodeadm-tracker

nodeadm init --skip run,preprocess --config-source file://config.yaml

nodeadm upgrade $TARGET_VERSION --skip run,pod-validation,node-validation --config-source file://config.yaml
assert::path-exists /usr/bin/containerd
assert::path-exists /usr/sbin/iptables
assert::path-exists /usr/local/bin/kubectl
VERSION_INFO=$(/usr/local/bin/kubectl version || true)
assert::is-substring "$VERSION_INFO" "v$TARGET_VERSION"
assert::path-exists /opt/cni/bin/
assert::path-exists /etc/eks/image-credential-provider/ecr-credential-provider
assert::path-exists /usr/local/bin/aws-iam-authenticator
assert::path-exists /usr/local/bin/aws_signing_helper
assert::files-equal /opt/aws/nodeadm-tracker expected-nodeadm-tracker
