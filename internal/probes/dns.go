package probes

import (
	"context"
	"net"
	"time"
)

// DNSResult holds the result of a DNS resolution test.
type DNSResult struct {
	Target      string
	Host        string
	ResolvedIPs []string
	Duration    time.Duration
	Error       error
}

// Resolve performs a DNS lookup for the given host with a hard 5-second timeout.
// net.LookupHost on Windows can block indefinitely without a context deadline.
func Resolve(host string) DNSResult {
	return ResolveTimeout(host, 5*time.Second)
}

// ResolveTimeout performs a DNS lookup with a configurable deadline.
func ResolveTimeout(host string, timeout time.Duration) DNSResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := &net.Resolver{}
	ips, err := resolver.LookupHost(ctx, host)
	duration := time.Since(start)

	return DNSResult{
		Target:      host,
		Host:        host,
		ResolvedIPs: ips,
		Duration:    duration,
		Error:       err,
	}
}
