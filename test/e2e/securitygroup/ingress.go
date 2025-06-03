package securitygroup

// IngessConfig represents a security group ingress configuration
type IngessConfig struct {
	Port        int    `json:"port"`
	Description string `json:"description,omitempty"`
}

// DefaultIngress returns the default list of ports to open
func DefaultIngress() []IngessConfig {
	return []IngessConfig{
		{
			Port:        10251,
			Description: "Metrics Server",
		},
		{
			Port:        8080,
			Description: "Kube State Metrics",
		},
		{
			Port:        9100,
			Description: "Prometheus Node Exporter",
		},
		{
			Port:        9403,
			Description: "Cert Manager",
		},
		{
			Port:        9402,
			Description: "Cert Manager CA Injector",
		},
		{
			Port:        10260,
			Description: "Cert Manager Webhook",
		},
	}
}
