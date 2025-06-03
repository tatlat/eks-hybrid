package network

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http/httpproxy"
)

const dialTimeout = 5 * time.Second

type ConnectionOptions struct {
	// ProxyFunc determines which proxy to use for a given request.
	// If nil, uses httpproxy.FromEnvironment().ProxyFunc()
	ProxyFunc func(*url.URL) (*url.URL, error)
}

type ConnectionOption func(*ConnectionOptions)

// this is overridable mainly for testing purposes to avoid using environment variables
// to manipulate the proxy environment and because the default ProxyFunc always skips the proxy
// for localhost/loopback addresses, it does not require that they be listed in the NO_PROXY
// environment variable. This makes testing difficult since we would need to manipulate the dns resolution
// on the test machine (ex: modify /etc/hosts).
func WithProxyFunc(f func(*url.URL) (*url.URL, error)) ConnectionOption {
	return func(o *ConnectionOptions) {
		o.ProxyFunc = f
	}
}

// CheckConnectionToHost checks if a connection can be established to the host
// specified in the URL.
func CheckConnectionToHost(ctx context.Context, targetURL url.URL, opts ...ConnectionOption) error {
	// Use default proxy function if none provided
	options := &ConnectionOptions{
		ProxyFunc: httpproxy.FromEnvironment().ProxyFunc(),
	}
	for _, opt := range opts {
		opt(options)
	}

	port := targetURL.Port()
	if port == "" && targetURL.Scheme == "https" {
		port = "443"
	}

	target := fmt.Sprintf("%s:%s", targetURL.Hostname(), port)

	proxyURL, err := options.ProxyFunc(&targetURL)
	if err != nil {
		return fmt.Errorf("getting proxy URL: %w", err)
	}

	host := target
	if proxyURL != nil {
		host = proxyURL.Host
	}

	conn, err := net.DialTimeout("tcp", host, dialTimeout)
	if err != nil {
		return fmt.Errorf("dialing %s: %w", host, err)
	}
	defer conn.Close()

	if proxyURL == nil {
		return nil
	}

	// The CONNECT method requests that the recipient establish a tunnel to
	// the destination origin server identified by the request-target and,
	// if successful, thereafter restrict its behavior to blind forwarding
	// of packets, in both directions, until the tunnel is closed.
	//
	// https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: target},
		Host:   target,
		Header: make(http.Header),
	}

	if err := req.Write(conn); err != nil {
		return fmt.Errorf("proxy: writing CONNECT request: %w", err)
	}

	// this will not be a typical HTTP response/body, but a tunnel response
	// this is before any tls negotiation, only the tunnel to the target is established
	// ex: HTTP/1.1 200 Connection Established
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return fmt.Errorf("proxy: reading CONNECT response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxy: returned status: %d", resp.StatusCode)
	}

	return nil
}
