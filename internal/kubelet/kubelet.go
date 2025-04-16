package kubelet

import "k8s.io/client-go/kubernetes"

// Kubelet groups several helpers so it can be injected in other
// packages without creating a dependency on this one and facilitating
// testing withouthaving to read the disk.
type Kubelet struct {
	Kubeconfig
}

func New() Kubelet {
	return Kubelet{}
}

// BuildClient builds a new Kubernetes client from the kubelet's kubeconfig.
func (k Kubelet) BuildClient() (kubernetes.Interface, error) {
	return k.Kubeconfig.BuildClient()
}

// KubeconfigPath returns the path to the kubelet's kubeconfig.
func (k Kubelet) KubeconfigPath() string {
	return k.Path()
}

// Version returns the version of the kubelet.
func (k Kubelet) Version() (string, error) {
	return GetKubeletVersion()
}
