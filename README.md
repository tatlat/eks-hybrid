# EKS Hybrid Nodes
## nodeadm

Bootstraps and initializes a node in an EKS cluster.

### Usage
#### Installation

To install all dependencies in the node:

**For SSM:**
```sh
nodeadm install -p ssm
```

**For IAM Roles Anywhere:**
```sh
nodeadm install -p iam-ra
```

#### Initialization
To initialize a node:
```
nodeadm init --config-source file://path/to/config.yaml
```

---

### Configuration

`nodeadm` uses a YAML configuration schema that will look familiar to Kubernetes users.

```yaml
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: my-cluster
    region: us-west-2
    apiServerEndpoint: https://example.com
    certificateAuthority: Y2VydGlmaWNhdGVBdXRob3JpdHk=
    cidr: 10.100.0.0/16
  hybrid:
    nodeName: my-node
    ssm: {}
    iamRolesAnywhere: {}
```

If you don't specify `apiServerEndpoint`, `certificateAuthority` or `cidr`, `nodeadm` will attempt to retrieve this data calling `DescribeCluster` on the EKS API.

```yaml
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: my-cluster
    region: us-west-2
  hybrid:
    nodeName: my-node
    ssm: {}
    iamRolesAnywhere: {}
```

You can provide this configuration in your machine's user data, either as-is or embedded within a MIME multi-part document:
```
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="BOUNDARY"

--BOUNDARY
Content-Type: application/node.eks.aws

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec: ...

--BOUNDARY--
```

A different source for the configuration object can be specified with the `--config-source` flag.


#### SSM

```yaml
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: my-cluster
    region: us-west-2
  hybrid:
    ssm:
      activationCode: <ssm-activation-code>
      activationId: <ssm-activation-id>
```

#### IAM Roles Anywhere

```yaml
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: my-cluster
    region: us-west-2
  hybrid:
    nodeName: my-node
    iamRolesAnywhere:
      trustAnchorArn: <trust-anchor-arn>
      profileArn: <profile-arn>
      roleArn: <node-role-arn>
```

## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more information.

## License

This project is licensed under the Apache-2.0 License.
