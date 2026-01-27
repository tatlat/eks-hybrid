#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

ARTIFACTS_BUCKET="$1"
BUCKET_PREFIX="$2"

SED=sed
if [ "$(uname -s)" = "Darwin" ]; then
    SED=gsed
fi

# *********************************************************************
# DO NOT EDIT this list unless you are sure you know what you are doing
# *********************************************************************

# expected list of files is intentional hardcoded and not generated
# to ensure we do not accidentally upload new files or remove files from the list  
# this will run after staging and prod releases
EXPECTED_FILES_FILE=$(mktemp)
cat <<EOF > ${EXPECTED_FILES_FILE}
ATTRIBUTION.txt
GIT_VERSION
bin/linux/amd64/nodeadm
bin/linux/amd64/nodeadm.gz
bin/linux/amd64/nodeadm.gz.sha256
bin/linux/amd64/nodeadm.gz.sha512
bin/linux/amd64/nodeadm.sha256
bin/linux/amd64/nodeadm.sha512
bin/linux/arm64/nodeadm
bin/linux/arm64/nodeadm.gz
bin/linux/arm64/nodeadm.gz.sha256
bin/linux/arm64/nodeadm.gz.sha512
bin/linux/arm64/nodeadm.sha256
bin/linux/arm64/nodeadm.sha512
EOF
# *********************************************************************
sort -o ${EXPECTED_FILES_FILE} ${EXPECTED_FILES_FILE}

echo "Validating release artifacts..."
# get a list of files via s3 cli
# ignore empty files since they are place holders for directories
if ! S3_FILES=$(aws s3api list-objects-v2  --bucket $ARTIFACTS_BUCKET --prefix $BUCKET_PREFIX  --query "Contents[?Size!=\`0\`].Key" --output text ); then
    echo "Failed to get list of files from S3"
    exit 1
fi

S3_FILES_FILE=$(mktemp)
# remove the bucket prefix from the list of files
for file in ${S3_FILES[@]}; do
    echo $file | $SED -e "s#^${BUCKET_PREFIX}/##" >> ${S3_FILES_FILE}
done

sort -o ${S3_FILES_FILE} ${S3_FILES_FILE}

if ! [ -s ${S3_FILES_FILE} ]; then
    echo "Actual files list is empty"
    exit 1
fi

if ! [ -s ${EXPECTED_FILES_FILE} ]; then
    echo "Expected files list is empty"
    exit 1
fi

if ! diff -q ${EXPECTED_FILES_FILE} ${S3_FILES_FILE} &>/dev/null; then
    echo "Artifacts directory on S3 does not matched expected!"
    diff -y ${EXPECTED_FILES_FILE} ${S3_FILES_FILE}
    exit 1
fi

echo "Release artifacts validated successfully"
