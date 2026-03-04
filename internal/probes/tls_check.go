package probes

import (
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

// TLSResult holds the result of a TLS handshake test.
type TLSResult struct {
	Target        string
	HandshakeTime time.Duration
	Version       uint16
	CipherSuite   uint16
	Success       bool
	Error         error
}

// CheckTLS performs a TLS handshake with the target.
// Both the dial and the Handshake() call are bounded by timeout.
func CheckTLS(host string, port int, timeout time.Duration) TLSResult {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()

	result := TLSResult{Target: target}

	dialer := &net.Dialer{Timeout: timeout}

	conf := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		ServerName:         host,
	}

	// tls.DialWithDialer covers the TCP connect + TLS overhead,
	// but conn.Handshake() needs its own deadline on the conn.
	rawConn, err := dialer.Dial("tcp", target)
	if err != nil {
		result.Error = err
		result.HandshakeTime = time.Since(start)
		return result
	}
	defer rawConn.Close()

	// Set a hard deadline so Handshake() can never block past it.
	deadline := time.Now().Add(timeout)
	if err := rawConn.SetDeadline(deadline); err != nil {
		result.Error = err
		result.HandshakeTime = time.Since(start)
		return result
	}

	tlsConn := tls.Client(rawConn, conf)
	defer tlsConn.Close()

	if err := tlsConn.Handshake(); err != nil {
		result.Error = err
		result.HandshakeTime = time.Since(start)
		return result
	}

	state := tlsConn.ConnectionState()
	result.HandshakeTime = time.Since(start)
	result.Version = state.Version
	result.CipherSuite = state.CipherSuite
	result.Success = true

	return result
}
