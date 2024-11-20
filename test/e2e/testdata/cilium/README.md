## Cilium template

You'll need to mirror the images from quay to our ecr repo. You can use ./mirror-cilium.sh $cilium_version for this. We want to pull from our own repository so the tests don't depend on the availability of the upstream images.
```sh
./mirror-cilium.sh v1.16.3
```

The cilium-template.yaml is generated with helm and the values.yaml is updated by running:
```sh
./update-manifests.sh v1.16.3
```
You can update the cilium version or any other param by re-running this command with the desired version and committing the changes.
