# Patching Moto in Integration Tests

This document describes how to temporarily patch moto in the integration test Dockerfile while waiting for upstream releases.

## Overview

Sometimes we need to switch to an unreleased moto or carry patches for moto while waiting for upstream releases. This document outlines the process for creating and applying patches to moto in our integration test environment.

## Steps to install unreleased moto

1. Update the Dockerfile to pip install from a specific commit:
   ```dockerfile
   # Install moto from specific commit
   RUN pip install --user 'moto[server] @ git+https://github.com/getmoto/moto.git@<commit-hash>'
   ```

2. Document the reason for using the specific commit in a comment above the installation line.

## Steps to patch moto

1. Create a `moto-patches` directory in `test/integration/infra/` if it doesn't exist:
   ```bash
   mkdir -p test/integration/infra/moto-patches
   ```

2. Generate a patch file, ideally open a PR upstream with the changes and store in the `moto-patches` folder.

3. Ideally, open a PR upstream with the changes and store the patch in the `moto-patches` folder while waiting for the PR to be merged.

4. Update the Dockerfile to apply the patch:
   ```dockerfile
   # Install moto from specific commit
   RUN pip install --user 'moto[server] @ git+https://github.com/getmoto/moto.git@<commit-hash>'

   # Copy and apply patches
   COPY test/integration/infra/moto-patches/*.patch /moto-patches/
   RUN patch -p1 -d /root/.local/lib/python*/site-packages/ < /moto-patches/your-patch.patch
   ```

## Best Practices

1. Name patch files with a descriptive name or PR number (e.g., `8710.patch`)
2. Document the reason for the patch in the patch file header
3. Include the upstream issue/PR reference if available
4. Remove patches once they are included in an upstream release
5. Keep patches minimal and focused on specific issues

## Removing Patches

Once a patch is included in an upstream release:
1. Remove the patch file from `test/integration/infra/moto-patches/`
2. Update the moto installation in the Dockerfile to use the new release
3. Remove the patch application command from the Dockerfile
4. Update any documentation or comments that reference the patch
