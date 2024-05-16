package kubelet

import (
	"github.com/aws/eks-hybrid/internal/util"
)

const caCertificatePath = "/etc/kubernetes/pki/ca.crt"

// Write the cluster certifcate authority to the filesystem where
// both kubelet and kubeconfig can read it
func writeClusterCaCert(caCert []byte) error {
	return util.WriteFileWithDir(caCertificatePath, caCert, kubeletConfigPerm)
}
