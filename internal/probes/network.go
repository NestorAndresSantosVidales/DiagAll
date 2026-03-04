package probes

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// HostResult represents an active host found in the network
type HostResult struct {
	IP          string
	ScanResults []ScanResult
}

// DiscoverHosts identifies active hosts in a CIDR range and scans them
func DiscoverHosts(cidr string, ports []int, timeout time.Duration) ([]HostResult, error) {
	ips, err := parseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var results []HostResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Worker pool for host discovery - limit concurrency to avoid overwhelming local stack
	concurrencyLimit := 20
	jobs := make(chan string, len(ips))

	for i := 0; i < concurrencyLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				if isHostUp(ip, 1*time.Second) {
					// Host is up, perform port scan
					portResults := PortScan(ip, ports, timeout)
					mu.Lock()
					results = append(results, HostResult{
						IP:          ip,
						ScanResults: portResults,
					})
					mu.Unlock()
				}
			}
		}()
	}

	for _, ip := range ips {
		jobs <- ip
	}
	close(jobs)
	wg.Wait()

	return results, nil
}

// isHostUp checks if a host is responsive by attempting to connect to common ports (80, 443, 22, 445)
func isHostUp(ip string, timeout time.Duration) bool {
	commonPorts := []int{80, 443, 22, 445, 3389}
	for _, port := range commonPorts {
		address := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// parseCIDR takes a CIDR string and returns a list of individual IP strings
func parseCIDR(cidr string) ([]string, error) {
	ipStr, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ipStr.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast addresses for /24 or smaller
	ones, _ := ipNet.Mask.Size()
	if ones <= 30 {
		if len(ips) > 2 {
			return ips[1 : len(ips)-1], nil
		}
	}

	return ips, nil
}

// inc increments an IP address
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
