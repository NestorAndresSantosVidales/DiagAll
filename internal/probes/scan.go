package probes

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ScanResult represents the status of a single port
type ScanResult struct {
	Port    int
	Open    bool
	Service string
	Banner  string
}

// PortScan performs a TCP Connect scan on a target host for a list of ports
func PortScan(target string, ports []int, timeout time.Duration) []ScanResult {
	var results []ScanResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a worker pool or just a goroutine per port if it's a small list
	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			res := scanPort(target, p, timeout)
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(port)
	}

	wg.Wait()
	return results
}

func scanPort(target string, port int, timeout time.Duration) ScanResult {
	address := net.JoinHostPort(target, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return ScanResult{Port: port, Open: false}
	}
	defer conn.Close()

	res := ScanResult{Port: port, Open: true}

	// Banner Grabbing
	conn.SetReadDeadline(time.Now().Add(timeout))
	buffer := make([]byte, 256)
	n, err := conn.Read(buffer)
	if err == nil && n > 0 {
		res.Banner = strings.TrimSpace(string(buffer[:n]))
		res.Service = identifyService(port, res.Banner)
	} else {
		res.Service = identifyService(port, "")
	}

	return res
}

func identifyService(port int, banner string) string {
	bannerLower := strings.ToLower(banner)

	// Simple heuristic mapping
	if strings.Contains(bannerLower, "ssh") {
		return "SSH"
	}
	if strings.Contains(bannerLower, "http") || strings.Contains(bannerLower, "nginx") || strings.Contains(bannerLower, "apache") {
		return "HTTP"
	}
	if strings.Contains(bannerLower, "mysql") {
		return "MySQL"
	}
	if strings.Contains(bannerLower, "rdesktop") || port == 3389 {
		return "RDP"
	}

	// Fallback to common ports if banner is empty or unknown
	switch port {
	case 21:
		return "FTP"
	case 22:
		return "SSH"
	case 23:
		return "Telnet"
	case 25:
		return "SMTP"
	case 53:
		return "DNS"
	case 80:
		return "HTTP"
	case 110:
		return "POP3"
	case 443:
		return "HTTPS"
	case 445:
		return "SMB"
	case 3306:
		return "MySQL"
	case 3389:
		return "RDP"
	case 5432:
		return "PostgreSQL"
	case 8080:
		return "HTTP-Proxy"
	}

	return "Unknown"
}
