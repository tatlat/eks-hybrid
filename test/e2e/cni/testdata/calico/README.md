## Calico template

You'll need to mirror the images from quay to our ecr repo. You can use ./mirror-calico.sh $calico_version for this. We want to pull from our own repository so the tests don't depend on the availability of the upstream images.
```sh
./mirror-calico.sh v3.29.0
```

The calico-template.yaml is updated by running:
```sh
./update-manifests.sh v3.29.0
```
