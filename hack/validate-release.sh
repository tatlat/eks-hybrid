#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# Required arguments
CLOUDFRONT_ID=$1
PROFILE=$2
VERSION_FILE=$3

echo "Starting release validation..."

# Create and wait for CloudFront invalidation
echo "Invalidating CloudFront cache..."
INVALIDATION_ID=$(aws cloudfront create-invalidation --distribution-id "${CLOUDFRONT_ID}" --paths "/releases/latest/bin/*" --profile "${PROFILE}" --query 'Invalidation.Id' --output text)
echo "Created invalidation with ID: ${INVALIDATION_ID}"

echo "Waiting for CloudFront invalidation to complete..."
while true; do
    STATUS=$(aws cloudfront get-invalidation --distribution-id "${CLOUDFRONT_ID}" --id "${INVALIDATION_ID}" --profile "${PROFILE}" --query 'Invalidation.Status' --output text)
    echo "Current invalidation status: ${STATUS}"
    if [ "${STATUS}" = "Completed" ]; then
        break
    elif [ "${STATUS}" = "Failed" ]; then
        echo "CloudFront invalidation failed!"
        exit 1
    fi
    sleep 10
done
echo "CloudFront invalidation completed successfully"

# Validate released version
echo "Validating released version..."
curl -L -o released_nodeadm https://hybrid-assets.eks.amazonaws.com/releases/latest/bin/linux/amd64/nodeadm
chmod +x released_nodeadm
RELEASED_VERSION=$(./released_nodeadm version)
EXPECTED_VERSION=$(cat "${VERSION_FILE}")

if [ "${RELEASED_VERSION}" != "${EXPECTED_VERSION}" ]; then
    echo "Version mismatch! Released version (${RELEASED_VERSION}) does not match expected version (${EXPECTED_VERSION})"
    exit 1
fi
echo "Version validation successful"

echo "Production release completed successfully"
echo "Version: ${VERSION_FILE}"
