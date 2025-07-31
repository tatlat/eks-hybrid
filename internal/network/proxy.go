package network

import (
	"os"

	"golang.org/x/net/http/httpproxy"
)

func IsProxyEnabled() bool {
	proxyEnv := httpproxy.FromEnvironment()
	return proxyEnv.HTTPProxy != "" || proxyEnv.HTTPSProxy != "" ||
		os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" ||
		os.Getenv("http_proxy") != "" || os.Getenv("https_proxy") != ""
}
