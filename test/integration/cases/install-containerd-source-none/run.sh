#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source /helpers.sh

mock::aws
wait::dbus-ready

# remove previously installed containerd to test installation via nodeadm
dnf remove -y containerd

nodeadm install 1.30  --credential-provider iam-ra --containerd-source none

# /usr/bin/containerd not exists means nodeadm did not install containerd from any source
assert::path-not-exist /usr/bin/containerd
assert::path-exists /usr/sbin/iptables
assert::path-exists /usr/bin/kubelet
assert::path-exists /usr/local/bin/kubectl
assert::path-exists /opt/cni/bin/
assert::path-exists /etc/eks/image-credential-provider/ecr-credential-provider
assert::path-exists /usr/local/bin/aws-iam-authenticator

assert::path-exists /usr/local/bin/aws_signing_helper

assert::files-equal /opt/aws/nodeadm-tracker expected-nodeadm-tracker
