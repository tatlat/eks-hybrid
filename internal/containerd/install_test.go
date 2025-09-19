package containerd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineContainerdVersionConstraint(t *testing.T) {
	tests := []struct {
		name               string
		kubernetesVersion  string
		expectedConstraint string
	}{
		{
			name:               "K8s 1.28 should use containerd 1.* constraint",
			kubernetesVersion:  "1.28.0",
			expectedConstraint: "1.*",
		},
		{
			name:               "K8s 1.29.0 should use containerd 1.* constraint",
			kubernetesVersion:  "1.29.0",
			expectedConstraint: "1.*",
		},
		{
			name:               "K8s 1.29.5 should use containerd 1.* constraint",
			kubernetesVersion:  "1.29.5",
			expectedConstraint: "1.*",
		},
		{
			name:               "K8s 1.30.0 should use no constraint (allows 2.x)",
			kubernetesVersion:  "1.30.0",
			expectedConstraint: "",
		},
		{
			name:               "K8s 1.30.1 should use no constraint (allows 2.x)",
			kubernetesVersion:  "1.30.1",
			expectedConstraint: "",
		},
		{
			name:               "K8s 1.30.5 should use no constraint (allows 2.x)",
			kubernetesVersion:  "1.30.5",
			expectedConstraint: "",
		},
		{
			name:               "K8s 1.31.0 should use no constraint (allows 2.x)",
			kubernetesVersion:  "1.31.0",
			expectedConstraint: "",
		},
		{
			name:               "K8s 1.32.0 should use no constraint (allows 2.x)",
			kubernetesVersion:  "1.32.0",
			expectedConstraint: "",
		},
		{
			name:               "K8s 2.0.0 should use no constraint (allows 2.x)",
			kubernetesVersion:  "2.0.0",
			expectedConstraint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineContainerdVersionConstraint(tt.kubernetesVersion)
			assert.Equal(t, tt.expectedConstraint, got,
				"determineContainerdVersionConstraint(%q) returned %q, expected %q",
				tt.kubernetesVersion, got, tt.expectedConstraint)
		})
	}
}
