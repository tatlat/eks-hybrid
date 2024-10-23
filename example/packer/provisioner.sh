#!/bin/bash
set -e

# check that credentials were provided 
if [ -z "$rhsm_username" ] || [ -z "$rhsm_password" ] || [ -z "$nodeadm_link" ] || [ -z "$auth_value" ] || [ -z "$rhel_version" ] || [ -z "$k8s_version" ]; then
    echo "Error: Please set rhsm_username and rhsm_password as environment variables."
    exit 1
fi

# register red hat subscription manager
sudo subscription-manager register --username="$rhsm_username" --password="$rhsm_password" --auto-attach

# enable required repos
sudo subscription-manager repos --enable=rhel-"$rhel_version"-for-x86_64-baseos-rpms
sudo subscription-manager repos --enable=rhel-"$rhel_version"-for-x86_64-appstream-rpms

# install curl
sudo dnf install -y curl

# download nodeadm
curl -o /tmp/nodeadm "$nodeadm_link"

chmod +x /tmp/nodeadm

sudo /tmp/nodeadm install "$k8s_version" --credential-provider "$auth_value" --containerd-source docker
