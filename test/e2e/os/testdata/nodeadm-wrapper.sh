#!/usr/bin/env bash
AWS_ENDPOINT_URL_EKS={{ .EKSEndpoint }} /usr/local/bin/nodeadm "$@"