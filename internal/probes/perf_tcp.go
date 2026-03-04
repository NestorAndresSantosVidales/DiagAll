package probes

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// TCPPerfResult holds the result of a TCP performance test.
type TCPPerfResult struct {
	Mode          string // "client" or "server"
	TotalBytes    int64
	Duration      time.Duration
	ThroughputBps float64
	Streams       int
	Error         error
}

// StartTCPServer starts a TCP server for performance testing.
// It accepts connections and reads data until closed or timeout.
func StartTCPServer(port int, duration time.Duration) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("TCP Server listening on port %d for %v...\n", port, duration)

	// Close listener after duration to stop accepting new connections
	// Note: This is a simple implementation. A robust one would handle graceful shutdown.
	go func() {
		time.Sleep(duration)
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Listener closed
			return nil
		}
		go handlePerfConn(conn)
	}
}

func handlePerfConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 32*1024)
	total := 0
	start := time.Now()
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			total += n
		}
		if err != nil {
			break
		}
	}
	duration := time.Since(start)
	bps := float64(total) / duration.Seconds()
	fmt.Printf("Received %d bytes in %v (%.2f MB/s) from %s\n", total, duration, bps/1024/1024, conn.RemoteAddr())
}

// PerfCallback is used to stream throughput updates
type PerfCallback func(bps float64)

// RunTCPClient runs a TCP performance test (client mode)
func RunTCPClient(host string, port int, streams int, duration time.Duration, onUpdate PerfCallback) TCPPerfResult {
	target := net.JoinHostPort(host, strconv.Itoa(port))
	var wg sync.WaitGroup

	// Shared stats
	var totalBytes int64
	var mu sync.Mutex

	start := time.Now()
	done := make(chan bool)

	// Interval reporter
	if onUpdate != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			var lastBytes int64
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					mu.Lock()
					current := totalBytes
					mu.Unlock()

					// Bytes since last tick
					diff := current - lastBytes
					lastBytes = current

					// Bits per second
					bps := float64(diff) * 8
					onUpdate(bps)
				}
			}
		}()
	}

	for i := 0; i < streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", target, 5*time.Second)
			if err != nil {
				// Count error?
				return
			}
			defer conn.Close()

			buf := make([]byte, 32*1024) // 32KB
			// Fill buffer
			for j := range buf {
				buf[j] = 'x'
			}

			// Send loop
			endTime := time.Now().Add(duration)
			for time.Now().Before(endTime) {
				n, err := conn.Write(buf)
				if err != nil {
					break
				}
				mu.Lock()
				totalBytes += int64(n)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	close(done)
	elapsed := time.Since(start)

	bps := float64(totalBytes) * 8 / elapsed.Seconds()

	return TCPPerfResult{
		Mode:          "client",
		TotalBytes:    totalBytes,
		Duration:      elapsed,
		ThroughputBps: bps,
		Streams:       streams,
	}
}
