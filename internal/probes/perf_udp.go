package probes

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// UDPPerfResult holds the result of a UDP performance test.
type UDPPerfResult struct {
	Mode        string
	PacketsSent int
	PacketsRecv int
	BytesRecv   int64
	LossRate    float64
	Jitter      time.Duration
	Duration    time.Duration
	Error       error
}

// StartUDPServer starts a UDP server for performance testing.
// It listens for packets and calculates stats.
func StartUDPServer(port int, duration time.Duration) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Printf("UDP Server listening on port %d for %v...\n", port, duration)

	// Set read deadline to duration + grace
	conn.SetReadDeadline(time.Now().Add(duration + 2*time.Second))

	buf := make([]byte, 2048)
	var count int
	var bytes int64

	// Jitter calculation (RFC 1889)
	// J = J + (|D(i-1,i)| - J)/16
	// D(i-1,i) = (R_i - S_i) - (R_{i-1} - S_{i-1})
	var lastTransit time.Duration
	var jitter float64

	startTime := time.Now()

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		recvTime := time.Now()
		bytes += int64(n)
		count++

		if n >= 8 {
			// Extract timestamp from first 8 bytes (int64 nanoseconds)
			sendTimeNs := int64(binary.BigEndian.Uint64(buf[:8]))
			sendTime := time.Unix(0, sendTimeNs)

			transit := recvTime.Sub(sendTime)
			if count > 1 {
				d := transit - lastTransit
				if d < 0 {
					d = -d
				}
				jitter = jitter + (float64(d)-jitter)/16.0
			}
			lastTransit = transit
		}

		if time.Since(startTime) > duration {
			break
		}
	}

	fmt.Printf("UDP Server: Received %d packets (%d bytes)\n", count, bytes)
	fmt.Printf("UDP Jitter: %v\n", time.Duration(jitter))
	return nil
}

// RunUDPClient sends UDP packets at a fixed rate.
// Simple implementation: send as fast as possible or with small sleep?
// Spec says "Client sends packets at fixed rate".
// We will send roughly 100 packets/sec for this test.
func RunUDPClient(host string, port int, duration time.Duration) UDPPerfResult {
	target := fmt.Sprintf("%s:%d", host, port)
	addr, _ := net.ResolveUDPAddr("udp", target)
	conn, err := net.DialUDP("udp", nil, addr)

	result := UDPPerfResult{
		Mode: "client",
	}

	if err != nil {
		result.Error = err
		return result
	}
	defer conn.Close()

	ticker := time.NewTicker(10 * time.Millisecond) // 100 pps
	defer ticker.Stop()

	endTime := time.Now().Add(duration)
	var sent int

	buf := make([]byte, 100) // Small payload

	for time.Now().Before(endTime) {
		<-ticker.C
		// Embed timestamp
		ts := time.Now().UnixNano()
		binary.BigEndian.PutUint64(buf[:8], uint64(ts))

		_, err := conn.Write(buf)
		if err != nil {
			result.Error = err
			break
		}
		sent++
	}

	result.PacketsSent = sent
	result.Duration = duration
	fmt.Printf("UDP Client: Sent %d packets in %v\n", sent, duration)

	return result
}
