# Hybrid Nodes Packer Template Usage

This template is provided to build Ubuntu and RHEL images with the required hybrid-nodes `nodeadm` installed and ready to run. An ISO image can be provided for Ubuntu 22.04/24.04 or RHEL 8/9 and the output can be either AMI, OVA, Qcow2 or Raw.

The repo includes supplemental kickstark files for Ubuntu and RHEL that help facilitate the OS install process, taken from the [Kubernetes-Sigs Repo](https://github.com/kubernetes-sigs/image-builder/tree/main/images/capi/packer).

## Prerequisites

Before using this template, ensure you have the following:

- Packer: version 1.11.0 or higher
- Packer Plugins:
    - Amazon Plugin: Version 1.2.8 or higher
    - vSphere Plugin: Version 1.4.0 or higher
    - QEMU Plugin: Version 1.x

## Overview

This template builds images for:
- AWS: Creates Amazon Machine Images (AMI) for Ubuntu 22.04/24.04 and RHEL 8/9. You can provide your AWS CLI profile and the resulting AMI will appear in the users account. 
- vSphere: Creates VMware templates using an ISO for Ubuntu and RHEL and appear in the specified folder.
- QEMU: Generates a Qcow2 or raw image for Ubuntu and RHEL.

These images have `nodeadm` installed and configured to the specifiec credential provider. 


## Required Environment Variables

Set the following environment variables before running the Packer build:

## Packer SSH Password
- PKR_SSH_PASSWORD: String. Packer uses the ssh_username and ssh_password variables to SSH into the created machine when provisioning. This needs to match the passwords used to create the initial user within the respective OS's `kickstart` or `user-data` files. The default is set as "builder" or "ubuntu" depending on the OS. When setting your password, make sure to change it within the corresponding `ks.cfg` or `user-data` file as well.  

## ISO Image and Checksum
- ISO_URL: String. URL of the ISO to use. Can be a web link to download from a server, or an absolute path to a local file
- ISO_CHECKSUM: String. Associated checksum for the supplied ISO. 

## AWS Configuration
- AWS_PROFILE: String. AWS profile for authentication.

## Credential Provider
- CREDENTIAL_PROVIDER: String. Authentication type for AWS temporary credentials. Valid values are:
    - ssm (default)
    - iam

## RHEL Subscription Manager
- RH_USERNAME: String. RHEL subscription manager username. 
- RH_PASSWORD: String. RHEL subscription manager password.

## RHEL Version Number
 - RHEL_VERSION: String. Rhel iso version being used. Can be 8 or 9.

## Kubernetes Version
 - K8S_VERSION: String. Kubernetes version to use. Must be major versions from 1.26 - 1.30.

## vSphere Configuration
- VSPHERE_SERVER: String. vSphere server address.
- VSPHERE_USER: String. vSphere username. 
- VSPHERE_PASSWORD: String. Vsphere password.
- VSPHERE_DATACENTER: String. vSphere datacenter name. 
- VSPHERE_CLUSTER: String. Vsphere cluster name. 
- VSPHERE_DATASTORE: String. vSphere datastore name.
- VSPHERE_NETWORK: String. vSphere network name.
- VSPHERE_OUTPUT_FOLDER: String. vSphere output folder for the templates. 

## QEMU Configuration
- PACKER_OUTPUT_FORMAT: String. Output format for the QEMU builder. Valid values are:
    - qcow2
    - raw

## Setup Instructions

1. Install Packer and the required plugins. 
2. Set up your environment variables:

```
export AWS_PROFILE="your-aws-profile"
export CREDENTIAL_PROVIDER="ssm"  # or "iam"
export RH_USERNAME="your-rhsm-username"
export RH_PASSWORD="your-rhsm-password"
export VSPHERE_SERVER="your-vsphere-server"
export VSPHERE_USER="your-vsphere-username"
export VSPHERE_PASSWORD="your-vsphere-password"
export VSPHERE_DATACENTER="your-datacenter"
export VSPHERE_CLUSTER="your-cluster"
export VSPHERE_DATASTORE="your-datastore"
export VSPHERE_NETWORK="your-network"
export VSPHERE_OUTPUT_FOLDER="your-output-folder"
export PACKER_OUTPUT_FORMAT="qcow2"  # or "raw"
export RHEL_VERSION="8" # 8 or 9
export K8S_VERSION="1.26" # Major version from 1.26 - 1.30
```
3. Validate the packer template
```
packer validate template.pkr.hcl
```

### Utilizing RHEL with Vsphere

In order to serve the included kickstart files with RHEL 8 and 9 on Vsphere, we need to convert it into a OEMDRV image and supply it as an ISO to boot from. You will need to manually upload the resulting ISO into the Vsphere datastore folder you specify under `iso_paths`. This guide assumes `[ YOUR_DATASTORE ] packer_cache/YOUR_RHEL_KS.iso`, you can set the `VSPHERE_DATACENTER` environment variable to set this at any time. Name your kickstart ISO as `rhel8_ks.iso` or `rhel9_ks.iso` to keep it consistent with the path used in this template. If you want to name it something else, make sure to change the filename in the path for the corresponding version. 

To create a kick start ISO, you will need to download `genisoimage`:

Ubuntu, Debian
```
sudo apt-get update
sudo apt-get install genisoimage
```

CentOS, RHEL, 
```
sudo yum install genisoimage
```

Arch Linux
```
sudo pacman -S cdrkit
```

Then, run the following command:

```
genisoimage -o YOUR_RHEL_KS.iso -V "OEMDRV" /PATH/TO/YOUR/KICKSTART.cfg
```



### Build Images

To build specific images, utilize the `-only` flag to leverage the general builder and specify the source needed. 


### AWS Images


#### Ubuntu 22.04 AMI
```
packer build -only=general-build.amazon-ebs.ubuntu22 template.pkr.hcl
```
#### Ubuntu 24.04 AMI
```
packer build -only=general-build.amazon-ebs.ubuntu24 template.pkr.hcl
```
#### RHEL 8
```
packer build -only=general-build.amazon-ebs.rhel8 template.pkr.hcl
```
#### RHEL 9
```
packer build -only=general-build.amazon-ebs.rhel9 template.pkr.hcl
```


### vSphere Images


#### Ubuntu 22.04 AMI
```
packer build -only=general-build.vsphere-iso.ubuntu22 template.pkr.hcl
```
#### Ubuntu 24.04 AMI
```
packer build -only=general-build.vsphere-iso.ubuntu24 template.pkr.hcl
```
#### RHEL 8
```
packer build -only=general-build.vsphere-iso.rhel8 template.pkr.hcl
```
#### RHEL 9
```
packer build -only=general-build.vsphere-iso.rhel9 template.pkr.hcl
```


### QEMU Images


#### Ubuntu 22.04 AMI
```
packer build -only=general-build.qemu.ubuntu22 template.pkr.hcl
```
#### Ubuntu 24.04 AMI
```
packer build -only=general-build.qemu.ubuntu24 template.pkr.hcl
```
#### RHEL 8
```
packer build -only=general-build.qemu.rhel8 template.pkr.hcl
```
#### RHEL 9
```
packer build -only=general-build.qemu.rhel9 template.pkr.hcl
```