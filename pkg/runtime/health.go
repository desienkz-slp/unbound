package runtime

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DNSHealthCheckResult contains latency and resolution verification data.
type DNSHealthCheckResult struct {
	Healthy      bool          `json:"healthy"`
	ServerIP     string        `json:"server_ip"`
	ServerPort   int           `json:"server_port"`
	Protocol     string        `json:"protocol"`
	QueryDomain  string        `json:"query_domain"`
	Latency      time.Duration `json:"latency"`
	ResolvedIPs  []string      `json:"resolved_ips,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// DNSHealthChecker queries a specific DNS server endpoint.
type DNSHealthChecker struct {
	Timeout time.Duration
}

// NewDNSHealthChecker creates a new health checker with standard default timeout.
func NewDNSHealthChecker(timeout time.Duration) *DNSHealthChecker {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &DNSHealthChecker{Timeout: timeout}
}

// CheckResolution probes the specified DNS server over UDP or TCP.
func (c *DNSHealthChecker) CheckResolution(ctx context.Context, serverIP string, serverPort int, protocol string, domain string) *DNSHealthCheckResult {
	if serverPort <= 0 {
		serverPort = 53
	}
	if protocol == "" {
		protocol = "udp"
	}
	if domain == "" {
		domain = "google.com"
	}

	res := &DNSHealthCheckResult{
		ServerIP:    serverIP,
		ServerPort:  serverPort,
		Protocol:    protocol,
		QueryDomain: domain,
	}

	targetAddr := net.JoinHostPort(serverIP, fmt.Sprintf("%d", serverPort))
	start := time.Now()

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: c.Timeout}
			return d.DialContext(ctx, protocol, targetAddr)
		},
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	ips, err := resolver.LookupHost(ctxTimeout, domain)
	res.Latency = time.Since(start)

	if err != nil {
		res.Healthy = false
		res.ErrorMessage = err.Error()
		return res
	}

	if len(ips) == 0 {
		res.Healthy = false
		res.ErrorMessage = "resolver returned empty IP list"
		return res
	}

	res.Healthy = true
	res.ResolvedIPs = ips
	return res
}
