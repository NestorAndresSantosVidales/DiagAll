package probes

import (
	"net"
	"strconv"
	"time"
)

// UDPResult holds the result of a UDP reachability test.
type UDPResult struct {
	Target    string
	Success   bool
	Error     error
	RTT       time.Duration
	Timestamp time.Time
	BytesSent int
	BytesRecv int
}

// ReachUDP attempts to send a UDP datagram to the target and wait for a reply (Echo mode).
// useful only if the target is running a UDP echo service or similar.
func ReachUDP(host string, port int, timeout time.Duration, payload []byte) UDPResult {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()

	result := UDPResult{
		Target:    target,
		Timestamp: start,
	}

	conn, err := net.DialTimeout("udp", target, timeout)
	if err != nil {
		result.Error = err
		return result
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		result.Error = err
		return result
	}

	if len(payload) == 0 {
		payload = []byte("diagall-udp-echo-test")
	}

	n, err := conn.Write(payload)
	if err != nil {
		result.Error = err
		return result
	}
	result.BytesSent = n

	buf := make([]byte, 1024)
	n, err = conn.Read(buf)
	if err != nil {
		result.Error = err
		return result
	}
	result.BytesRecv = n
	result.RTT = time.Since(start)
	result.Success = true

	return result
}
