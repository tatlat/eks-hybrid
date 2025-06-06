package network_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/network"
	"github.com/aws/eks-hybrid/internal/test"
)

type serverFunc func(*testing.T) (net.Listener, string)

type proxyBehavior int

const (
	proxyBehaviorSuccess proxyBehavior = iota
	proxyBehaviorBadGateway
	proxyBehaviorNoResponse
	proxyBehaviorBadStatus
)

// mockProxyServer creates a test proxy server that handles CONNECT requests and forwards to target
func mockProxyServer(t *testing.T, targetHost string, behavior proxyBehavior) (net.Listener, string) {
	t.Helper()
	g := NewGomegaWithT(t)

	proxy := test.NewHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Method).To(Equal(http.MethodConnect), "expected CONNECT request")

		// Hijack the connection to establish the tunnel
		hijacker, ok := w.(http.Hijacker)
		g.Expect(ok).To(BeTrue(), "webserver doesn't support hijacking")

		clientConn, _, err := hijacker.Hijack()
		g.Expect(err).NotTo(HaveOccurred(), "hijacking failed")
		defer clientConn.Close()

		switch behavior {
		case proxyBehaviorBadStatus:
			// Return a non-200 status
			_, err := clientConn.Write([]byte("HTTP/1.1 403 Forbidden\r\n\r\n"))
			g.Expect(err).NotTo(HaveOccurred(), "writing response failed")
			return
		case proxyBehaviorNoResponse:
			// Don't write any response, just close
			return
		case proxyBehaviorBadGateway:
			// Return 502 to simulate proxy returning a bad gateway error
			_, err := clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			g.Expect(err).NotTo(HaveOccurred(), "writing response failed")
			return
		}

		// Forward the connection to the target
		targetConn, err := net.Dial("tcp", targetHost)
		if err != nil {
			_, err := clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			g.Expect(err).NotTo(HaveOccurred(), "writing response failed")
			return
		}
		defer targetConn.Close()

		// Send 200 Connection Established
		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		g.Expect(err).NotTo(HaveOccurred(), "writing response failed")
	})

	proxyURL, err := url.Parse(proxy.URL)
	g.Expect(err).NotTo(HaveOccurred(), "failed to parse proxy URL")

	return proxy.Listener, proxyURL.Host
}

// mockTargetServer creates a test server that accepts and immediately closes connections
func mockTargetServer(t *testing.T) (net.Listener, string) {
	t.Helper()
	g := NewGomegaWithT(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).NotTo(HaveOccurred(), "failed to create listener")

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Accept and immediately close the connection
			conn.Close()
		}
	}()

	return listener, listener.Addr().String()
}

// mockMisbehavingTargetServer creates a test server that does not respond to the connection
func mockMisbehavingTargetServer(t *testing.T) (net.Listener, string) {
	t.Helper()
	g := NewGomegaWithT(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).NotTo(HaveOccurred(), "failed to create listener")
	// immediately close the listener to simulate a misbehaving target server
	listener.Close()

	return listener, listener.Addr().String()
}

func TestCheckConnectionToHost(t *testing.T) {
	tests := []struct {
		name           string
		useProxy       bool
		targetServer   serverFunc
		proxyBehavior  proxyBehavior
		proxyShouldErr bool
		wantErr        string
	}{
		{
			name:         "successful direct connection",
			useProxy:     false,
			targetServer: mockTargetServer,
		},
		{
			name:          "successful proxy connection",
			useProxy:      true,
			targetServer:  mockTargetServer,
			proxyBehavior: proxyBehaviorSuccess,
		},
		{
			name:           "proxy connection failure - invalid URL",
			useProxy:       true,
			targetServer:   mockTargetServer,
			proxyShouldErr: true,
			wantErr:        "dialing invalid-proxy:8080",
		},
		{
			name:          "proxy connection failure - bad gateway",
			useProxy:      true,
			targetServer:  mockMisbehavingTargetServer,
			proxyBehavior: proxyBehaviorBadGateway,
			wantErr:       "proxy: returned status: 502",
		},
		{
			name:          "proxy connection failure - forbidden",
			useProxy:      true,
			targetServer:  mockTargetServer,
			proxyBehavior: proxyBehaviorBadStatus,
			wantErr:       "proxy: returned status: 403",
		},
		{
			name:          "proxy connection failure - no response",
			useProxy:      true,
			targetServer:  mockTargetServer,
			proxyBehavior: proxyBehaviorNoResponse,
			wantErr:       "proxy: reading CONNECT response",
		},
		{
			name:         "target connection failure direct",
			useProxy:     false,
			targetServer: mockMisbehavingTargetServer,
			wantErr:      "dialing",
		},
		{
			name:           "proxy function error",
			useProxy:       true,
			targetServer:   mockTargetServer,
			proxyShouldErr: true,
			wantErr:        "getting proxy URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			// Always set up the target server
			targetServer, targetHost := tt.targetServer(t)
			defer targetServer.Close()

			// Set up proxy if needed
			var opts []network.ConnectionOption
			if tt.useProxy {
				proxyServer, proxyHost := mockProxyServer(t, targetHost, tt.proxyBehavior)
				defer proxyServer.Close()
				if tt.proxyShouldErr {
					// Return an invalid proxy URL or error
					opts = append(opts, network.WithProxyFunc(func(*url.URL) (*url.URL, error) {
						if tt.wantErr == "getting proxy URL" {
							return nil, fmt.Errorf("proxy function error")
						}
						return &url.URL{Host: "invalid-proxy:8080"}, nil
					}))
				} else {
					// Return our test proxy URL
					opts = append(opts, network.WithProxyFunc(func(*url.URL) (*url.URL, error) {
						return &url.URL{
							Scheme: "http",
							Host:   proxyHost,
						}, nil
					}))
				}
			}

			targetURL := url.URL{
				Scheme: "https",
				Host:   targetHost,
			}

			err := network.CheckConnectionToHost(context.Background(), targetURL, opts...)
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
