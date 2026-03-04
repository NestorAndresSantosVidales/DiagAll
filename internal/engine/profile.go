package engine

import (
	"fmt"
	"time"

	"diagall/internal/probes"
	"diagall/internal/report"
)

type Logger func(string)

// RunProfile executes a predefined set of tests and returns session data.
func RunProfile(name string, target string, log Logger) report.SessionData {
	if log == nil {
		log = func(s string) { fmt.Println(s) }
	} // Default to stdout if nil

	log(fmt.Sprintf("Running Profile: %s against %s\n", name, target))

	session := report.SessionData{
		ID:        fmt.Sprintf("session_%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Target:    target,
		Profile:   name,
	}

	switch name {
	case "wan":
		session.Results = runWANProfile(target, log)
	case "vpn":
		session.Results = runVPNProfile(target, log)
	default:
		log(fmt.Sprintf("Unknown profile: %s. Available: wan, vpn\n", name))
	}

	return session
}

func runWANProfile(target string, log Logger) []report.TestResult {
	var results []report.TestResult

	// 1. DNS Resolve
	log("[1/5] DNS Resolution...")
	dnsRes := probes.Resolve(target)
	res := report.TestResult{Name: "DNS Resolution", Status: "PASS", Metric: fmt.Sprintf("%v", dnsRes.Duration)}
	if dnsRes.Error != nil {
		res.Status = "FAIL"
		res.Details = dnsRes.Error.Error()
		log(fmt.Sprintf("FAIL: %v", dnsRes.Error))
	} else {
		res.Details = fmt.Sprintf("Resolved to %v", dnsRes.ResolvedIPs)
		log(fmt.Sprintf("PASS: Resolved to %v in %v", dnsRes.ResolvedIPs, dnsRes.Duration))
	}
	results = append(results, res)

	// 2. TCP Reach :443
	host := target
	port := 443

	log(fmt.Sprintf("[2/5] TCP Reachability to port %d...", port))
	reachRes := probes.ReachTCP(host, port, 2*time.Second)
	res = report.TestResult{Name: "TCP Reachability", Status: "PASS", Metric: fmt.Sprintf("%v", reachRes.RTT)}
	if reachRes.Success {
		log(fmt.Sprintf("PASS: RTT %v", reachRes.RTT))
	} else {
		res.Status = "FAIL"
		res.Details = reachRes.Error.Error()
		log(fmt.Sprintf("FAIL: %v", reachRes.Error))
	}
	results = append(results, res)

	// 3. TCP Trace
	log("[3/5] TCP Path Discovery...")
	hops, err := probes.TraceTCP(host, port, 15, 1*time.Second, func(h probes.TraceHop) {
		status := "OK"
		if h.Timeout {
			status = "*"
		}
		if h.Error != nil && !h.Timeout {
			status = "ERR"
		}
		log(fmt.Sprintf("Hop %d: %v (%s) %s", h.TTL, h.Host, h.RTT, status))
	})
	res = report.TestResult{Name: "TCP Trace", Status: "PASS", Metric: fmt.Sprintf("%d hops", len(hops))}
	if err != nil {
		res.Status = "FAIL"
		res.Details = err.Error()
		log(fmt.Sprintf("FAIL: %v", err))
	} else {
		res.Details = fmt.Sprintf(" reached in %d hops", len(hops))
		log(fmt.Sprintf("DONE: %d hops reached", len(hops)))
	}
	results = append(results, res)

	// 4. TLS Check
	log("[4/5] TLS Handshake...")
	tlsRes := probes.CheckTLS(host, port, 5*time.Second)
	res = report.TestResult{Name: "TLS Check", Status: "PASS", Metric: fmt.Sprintf("%v", tlsRes.HandshakeTime)}
	if tlsRes.Success {
		res.Details = fmt.Sprintf("Ver: %x, Cipher: %x", tlsRes.Version, tlsRes.CipherSuite)
		log(fmt.Sprintf("PASS: %v, Cipher: %x", tlsRes.HandshakeTime, tlsRes.CipherSuite))
	} else {
		res.Status = "FAIL"
		res.Details = tlsRes.Error.Error()
		log(fmt.Sprintf("FAIL: %v", tlsRes.Error))
	}
	results = append(results, res)

	// 5. Perf
	log("[5/5] Performance Test (Skipped)")
	results = append(results, report.TestResult{Name: "Performance", Status: "SKIP", Details: "Requires server"})

	return results
}

func runVPNProfile(target string, log Logger) []report.TestResult {
	var results []report.TestResult

	// 1. DNS Resolve internal
	log("[1/4] Internal DNS Resolution...")
	dnsRes := probes.Resolve(target)
	res := report.TestResult{Name: "DNS Resolution", Status: "PASS", Metric: fmt.Sprintf("%v", dnsRes.Duration)}
	if dnsRes.Error != nil {
		res.Status = "FAIL"
		res.Details = dnsRes.Error.Error()
		log(fmt.Sprintf("FAIL: %v", dnsRes.Error))
	} else {
		res.Details = fmt.Sprintf("Resolved to %v", dnsRes.ResolvedIPs)
		log(fmt.Sprintf("PASS: Resolved to %v", dnsRes.ResolvedIPs))
	}
	results = append(results, res)

	// 2. TCP Reach internal
	port := 443
	log(fmt.Sprintf("[2/4] Internal Service Reachability (%d)...", port))
	reachRes := probes.ReachTCP(target, port, 2*time.Second)
	res = report.TestResult{Name: "TCP Reachability", Status: "PASS", Metric: fmt.Sprintf("%v", reachRes.RTT)}
	if reachRes.Success {
		log(fmt.Sprintf("PASS: RTT %v", reachRes.RTT))
	} else {
		res.Status = "FAIL"
		res.Details = reachRes.Error.Error()
		log(fmt.Sprintf("FAIL: %v", reachRes.Error))
	}
	results = append(results, res)

	// 3. Trace
	log("[3/4] Path Trace...")
	hops, _ := probes.TraceTCP(target, port, 20, 1*time.Second, func(h probes.TraceHop) {
		log(fmt.Sprintf("Hop %d: %v %v", h.TTL, h.Host, h.RTT))
	})
	res = report.TestResult{Name: "TCP Trace", Status: "PASS", Metric: fmt.Sprintf("%d hops", len(hops))}
	log(fmt.Sprintf("Done: %d hops", len(hops)))
	results = append(results, res)

	// 4. Perf
	log("[4/4] Performance (Skipped)")
	results = append(results, report.TestResult{Name: "Performance", Status: "SKIP", Details: "Requires server"})

	return results
}
