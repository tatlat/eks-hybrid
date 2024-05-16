package system

import "github.com/aws/eks-hybrid/internal/api"

type SystemAspect interface {
	Name() string
	Setup(*api.NodeConfig) error
}
