#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# Required arguments
PROD_BUCKET=$1
PROFILE=$2
VERSION=$3

echo "Starting nodeadm release process..."

# Upload to production
echo "Uploading nodeadm to production..."
for ARCH in amd64 arm64; do
  aws s3 cp --no-progress _bin/${ARCH}/nodeadm "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/${ARCH}/nodeadm" --profile "${PROFILE}"
  aws s3 cp --no-progress _bin/${ARCH}/nodeadm.gz "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/${ARCH}/nodeadm.gz" --profile "${PROFILE}" --content-encoding gzip
done

# Generate and upload checksums
echo "Generating and uploading nodeadm checksums..."
for ARCH in amd64 arm64; do
  for FILE in nodeadm nodeadm.gz; do
    for CHECKSUM in sha256 sha512; do
      ${CHECKSUM}sum _bin/${ARCH}/${FILE} > _bin/${ARCH}/${FILE}.${CHECKSUM}
      aws s3 cp --no-progress _bin/${ARCH}/${FILE}.${CHECKSUM} "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/${ARCH}/${FILE}.${CHECKSUM}" --profile "${PROFILE}"
    done
  done
done

# Update latest symlinks
echo "Updating latest symlinks for nodeadm..."
aws s3 sync --no-progress s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/ s3://${PROD_BUCKET}/releases/latest/bin/linux/ --profile "${PROFILE}"

# Generate and upload attribution
echo "Generating and uploading attribution..."
make generate-attribution
aws s3 cp --no-progress ATTRIBUTION.txt "s3://${PROD_BUCKET}/releases/${VERSION}/ATTRIBUTION.txt" --profile "${PROFILE}"
aws s3 cp --no-progress ATTRIBUTION.txt "s3://${PROD_BUCKET}/releases/latest/ATTRIBUTION.txt" --profile "${PROFILE}"

echo "Release process completed successfully"
