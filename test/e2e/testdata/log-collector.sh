#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

LOGS_UPLOAD_NAME="$1"

declare -A LOGS_UPLOAD_URLS=()
{{ range $url := .LogsUploadUrls }}
LOGS_UPLOAD_URLS["{{ $url.Name }}"]="{{ $url.Url }}"
{{- end }}

if [ ! ${LOGS_UPLOAD_URLS[$LOGS_UPLOAD_NAME]+1} ]; then
    # no presigned url for name
    exit
fi

LOG_SCRIPT_URL="https://raw.githubusercontent.com/jaxesn/amazon-eks-ami/refs/heads/jgw/hybrid-script/log-collector-script/linux/eks-log-collector.sh"

curl -s --retry 5  $LOG_SCRIPT_URL -o /tmp/eks-log-collector.sh

bash /tmp/eks-log-collector.sh
# do not overwrite if the file is already there
# s3 will return a 412 PreconditionFailed if the file already exists
HTTP_CODE=$(curl --header 'If-None-Match: *' -w "%{http_code}" -s -o /dev/null --retry 5 --request PUT --upload-file /var/log/eks_* "${LOGS_UPLOAD_URLS[$LOGS_UPLOAD_NAME]}" )
rm /var/log/eks_*

if [[ ${HTTP_CODE} -lt 200 || ${HTTP_CODE} -gt 299 ]] ; then
    exit 1
fi
