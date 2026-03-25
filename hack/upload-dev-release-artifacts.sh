#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

ARTIFACTS_BUCKET="$1"

if [ "${SKIP_LATEST_UPLOAD:-false}" != "true" ]; then
  mkdir -p _bin/latest/bin/linux/{amd64,arm64}
  cp _bin/{ATTRIBUTION.txt,GIT_VERSION} _bin/latest/
  cp _bin/amd64/nodeadm{,.gz,.sha256,.sha512,.gz.sha256,.gz.sha512} _bin/latest/bin/linux/amd64/
  cp _bin/arm64/nodeadm{,.gz,.sha256,.sha512,.gz.sha256,.gz.sha512} _bin/latest/bin/linux/arm64/

  # uploading nodeadm.gz files separately to ensure the content-encoding/disposition is applied
  aws s3 sync --no-progress --exclude "*nodeadm.gz" _bin/latest/ s3://${ARTIFACTS_BUCKET}/latest/
  aws s3 sync --no-progress --include "*nodeadm.gz" --content-encoding gzip --content-disposition "attachment; filename=\"nodeadm\"" _bin/latest/ s3://${ARTIFACTS_BUCKET}/latest/
fi

# create artifacts zip for CodePipeline S3 source 
ARTIFACT_KEY="${ARTIFACT_KEY:-artifacts.zip}"
echo "Creating ${ARTIFACT_KEY} for CodePipeline S3 source..."
mkdir -p _bin/artifacts/_bin/{amd64,arm64}

# copy ALL binaries (nodeadm + test binaries) - same structure as build output
cp _bin/amd64/* _bin/artifacts/_bin/amd64/
cp _bin/arm64/* _bin/artifacts/_bin/arm64/

# copy buildspecs and scripts needed to run tests
mkdir -p _bin/artifacts/buildspecs _bin/artifacts/hack _bin/artifacts/test/e2e/cni/testdata
cp -r buildspecs/* _bin/artifacts/buildspecs/
cp -r hack/* _bin/artifacts/hack/
cp -r test/e2e/cni/testdata/* _bin/artifacts/test/e2e/cni/testdata/

# zip and upload to bucket root
(cd _bin/artifacts && zip -r ../${ARTIFACT_KEY} .)
aws s3 cp --no-progress _bin/${ARTIFACT_KEY} s3://${ARTIFACTS_BUCKET}/${ARTIFACT_KEY}
rm -rf _bin/artifacts _bin/${ARTIFACT_KEY}
echo "${ARTIFACT_KEY} uploaded successfully to s3://${ARTIFACTS_BUCKET}/${ARTIFACT_KEY}"
