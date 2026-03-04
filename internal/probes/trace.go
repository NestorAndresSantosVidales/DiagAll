package probes

import (
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"
)

// TraceHop represents a single hop in the TCP trace
type TraceHop struct {
	TTL     int
	Host    string
	RTT     time.Duration
	Success bool
	Final   bool
	Timeout bool
	Error   error
}

// TraceCallback is a function called for each hop discovered.
type TraceCallback func(TraceHop)

// TraceTCP performs a TCP traceroute-like operation by increasing TTL.
// It accepts an optional callback to stream results.
func TraceTCP(host string, port int, maxTTL int, timeout time.Duration, onHop TraceCallback) ([]TraceHop, error) {
	var hops []TraceHop

	// Resolve IP first to avoid repeated lookups
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP found for %s", host)
	}
	destIP := ips[0].String()

	for ttl := 1; ttl <= maxTTL; ttl++ {
		hop := TraceHop{TTL: ttl}
		start := time.Now()

		d := net.Dialer{
			Timeout: timeout,
			Control: func(network, address string, c syscall.RawConn) error {
				var opErr error
				err := c.Control(func(fd uintptr) {
					// Set TTL
					setErr := syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
					if setErr != nil {
						// IPv6 fallback
						syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IPV6, 4, ttl)
					}
					opErr = setErr
				})
				if err != nil {
					return err
				}
				return opErr
			},
		}

		target := net.JoinHostPort(destIP, strconv.Itoa(port))
		conn, err := d.Dial("tcp", target)

		hop.RTT = time.Since(start)

		if err == nil {
			hop.Success = true
			hop.Final = true
			hop.Host = conn.RemoteAddr().String()
			conn.Close()

			hops = append(hops, hop)
			if onHop != nil {
				onHop(hop)
			}
			break
		} else {
			hop.Timeout = true
			hop.Error = err
			hops = append(hops, hop)
			if onHop != nil {
				onHop(hop)
			}
		}
	}
	return hops, nil
}
