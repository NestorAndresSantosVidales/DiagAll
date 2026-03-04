package probes

import (
	"net"
	"strconv"
	"time"
)

// TCPResult holds the result of a TCP reachability test.
type TCPResult struct {
	Target    string
	Success   bool
	Error     error
	RTT       time.Duration
	Timestamp time.Time
}

// ReachTCP attempts to establish a TCP connection to the target host:port within the specified timeout.
func ReachTCP(host string, port int, timeout time.Duration) TCPResult {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()

	conn, err := net.DialTimeout("tcp", target, timeout)
	rtt := time.Since(start)

	result := TCPResult{
		Target:    target,
		Timestamp: start,
		RTT:       rtt,
	}

	if err != nil {
		result.Success = false
		result.Error = err
		return result
	}

	defer conn.Close()
	result.Success = true
	return result
}
