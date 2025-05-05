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
aws s3 cp --no-progress _bin/amd64/nodeadm "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/amd64/nodeadm" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/arm64/nodeadm "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/arm64/nodeadm" --profile "${PROFILE}"

# Generate and upload checksums
echo "Generating and uploading nodeadm checksums..."
# AMD64 checksums
sha256sum _bin/amd64/nodeadm > _bin/amd64/nodeadm.sha256
aws s3 cp --no-progress _bin/amd64/nodeadm.sha256 "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/amd64/nodeadm.sha256" --profile "${PROFILE}"
sha512sum _bin/amd64/nodeadm > _bin/amd64/nodeadm.sha512
aws s3 cp --no-progress _bin/amd64/nodeadm.sha512 "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/amd64/nodeadm.sha512" --profile "${PROFILE}"

# ARM64 checksums
sha256sum _bin/arm64/nodeadm > _bin/arm64/nodeadm.sha256
aws s3 cp --no-progress _bin/arm64/nodeadm.sha256 "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/arm64/nodeadm.sha256" --profile "${PROFILE}"
sha512sum _bin/arm64/nodeadm > _bin/arm64/nodeadm.sha512
aws s3 cp --no-progress _bin/arm64/nodeadm.sha512 "s3://${PROD_BUCKET}/releases/${VERSION}/bin/linux/arm64/nodeadm.sha512" --profile "${PROFILE}"

# Update latest symlinks
echo "Updating latest symlinks for nodeadm..."
aws s3 cp --no-progress _bin/amd64/nodeadm "s3://${PROD_BUCKET}/releases/latest/bin/linux/amd64/nodeadm" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/arm64/nodeadm "s3://${PROD_BUCKET}/releases/latest/bin/linux/arm64/nodeadm" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/amd64/nodeadm.sha256 "s3://${PROD_BUCKET}/releases/latest/bin/linux/amd64/nodeadm.sha256" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/arm64/nodeadm.sha256 "s3://${PROD_BUCKET}/releases/latest/bin/linux/arm64/nodeadm.sha256" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/amd64/nodeadm.sha512 "s3://${PROD_BUCKET}/releases/latest/bin/linux/amd64/nodeadm.sha512" --profile "${PROFILE}"
aws s3 cp --no-progress _bin/arm64/nodeadm.sha512 "s3://${PROD_BUCKET}/releases/latest/bin/linux/arm64/nodeadm.sha512" --profile "${PROFILE}"

# Generate and upload attribution
echo "Generating and uploading attribution..."
make generate-attribution
aws s3 cp --no-progress ATTRIBUTION.txt "s3://${PROD_BUCKET}/releases/${VERSION}/ATTRIBUTION.txt" --profile "${PROFILE}"
aws s3 cp --no-progress ATTRIBUTION.txt "s3://${PROD_BUCKET}/releases/latest/ATTRIBUTION.txt" --profile "${PROFILE}"

echo "Release process completed successfully"
