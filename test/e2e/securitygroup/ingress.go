package securitygroup

// IngessConfig represents a security group ingress configuration
type IngessConfig struct {
	Port    int    `json:"port"`
	AppName string `json:"appName"`
}

// DefaultIngress returns the default list of ports to open
func DefaultIngress() []IngessConfig {
	return []IngessConfig{
		{
			Port:    10251,
			AppName: "MetricsServer",
		},
		{
			Port:    8080,
			AppName: "KubeStateMetrics",
		},
		{
			Port:    9100,
			AppName: "PrometheusNodeExporter",
		},
		{
			Port:    9403,
			AppName: "CertManager",
		},
		{
			Port:    9402,
			AppName: "CertManagerCAInjector",
		},
		{
			Port:    10260,
			AppName: "CertManagerWebhook",
		},
	}
}
