package diagnosis

import (
	"context"
	"diagall/internal/ai"
	"fmt"
	"regexp"
	"strings"
)

// Severity levels
const (
	SeverityInfo     = "INFO"
	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

// Finding represents a single diagnostic observation.
type Finding struct {
	ID                 string   `json:"id"`
	Category           string   `json:"category"`
	Severity           string   `json:"severity"`
	Confidence         float64  `json:"confidence"` // 0.0 to 1.0
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Evidence           []string `json:"evidence"`
	ProbableCauses     []string `json:"probable_causes"`
	RecommendedActions []string `json:"recommended_actions"`
}

// AnalysisResult holds all findings for a session.
type AnalysisResult struct {
	SessionID string    `json:"session_id"`
	Findings  []Finding `json:"findings"`
	Summary   string    `json:"summary"`
	NextSteps []string  `json:"next_steps"`
}

// Engine is the main entry point for diagnosis.
type Engine struct {
	PrivacyMode bool // mask IPs/hostnames in output
	AIExpert    ai.Expert
}

// NewEngine creates a new diagnostic engine.
func NewEngine() *Engine {
	return &Engine{
		AIExpert: ai.NewLocalManager("."),
	}
}

// InputData holds all measurable data for analysis.
type InputData struct {
	// Reachability
	ReachabilitySuccess bool
	ReachabilityRTT     float64 // ms
	ReachabilityError   string

	// DNS
	DNSSuccess  bool
	DNSDuration float64 // ms

	// TLS
	TLSSuccess       bool
	TLSHandshakeTime float64 // ms

	// Path (per-hop data)
	Hops []HopData

	// Performance
	ThroughputMbps float64
	UDPLossPct     float64
	TCPSuccess     bool
	UDPSuccess     bool
	Streams        []StreamData

	// Jitter
	P95RTT float64
	AvgRTT float64

	// Scan Results
	ScanResults []ScanResult

	// Discovery Results
	DiscoveryResults []HostResult
}

// HostResult mirrors probes.HostResult
type HostResult struct {
	IP          string
	ScanResults []ScanResult
}

// ScanResult mirrors probes.ScanResult to avoid circular imports if needed,
// but here we just use the data.
type ScanResult struct {
	Port    int
	Open    bool
	Service string
	Banner  string
}

// HopData represents a single path hop's metrics.
type HopData struct {
	TTL  int
	Host string
	Loss float64
	RTT  float64
	P95  float64
}

// StreamData holds per-stream performance data.
type StreamData struct {
	ID         int
	Throughput float64
}

// Analyze runs all heuristics and produces findings + narrative.
func (e *Engine) Analyze(data InputData) AnalysisResult {
	ctx := context.Background() // Use background for now
	var findings []Finding

	findings = append(findings, e.analyzeReachability(data)...)
	findings = append(findings, e.analyzeDNS(data)...)
	findings = append(findings, e.analyzeTLS(data)...)
	findings = append(findings, e.analyzePath(data)...)
	findings = append(findings, e.analyzePerformance(data)...)
	findings = append(findings, e.analyzeQoS(data)...)
	findings = append(findings, e.analyzeJitter(data)...)
	findings = append(findings, e.analyzeStreams(data)...)
	findings = append(findings, e.analyzeScan(data)...)
	findings = append(findings, e.analyzeDiscovery(data)...)

	if len(findings) == 0 {
		findings = append(findings, Finding{
			ID:          "ALL_GOOD_01",
			Category:    "General",
			Severity:    SeverityInfo,
			Confidence:  0.95,
			Title:       "No Issues Detected",
			Description: "All monitored metrics are within normal thresholds.",
			Evidence:    []string{"All probes completed successfully"},
		})
	}

	nr := &NarrativeEngine{privacyMode: e.PrivacyMode}

	// Check for critical base failures (DNS or Reachability)
	baseFailure := !data.DNSSuccess || !data.ReachabilitySuccess

	summary := nr.GenerateSummary(findings, baseFailure)
	nextSteps := nr.GenerateNextSteps(findings, baseFailure)

	// NEW: Augment summary with AI Expert advice
	if e.AIExpert != nil {
		input := fmt.Sprintf("Summary: %s\nFindings: %v", summary, findings)
		aiSummary, err := e.AIExpert.Analyze(ctx, input)
		if err == nil {
			summary = fmt.Sprintf("%s\n\n---\n🤖 **Expert Analysis:**\n%s", summary, aiSummary)
		}
	}

	return AnalysisResult{
		Findings:  findings,
		Summary:   summary,
		NextSteps: nextSteps,
	}
}

// ─── Heuristic Analyzers ──────────────────────────────────────────────────────

func (e *Engine) analyzeReachability(data InputData) []Finding {
	var findings []Finding
	if !data.ReachabilitySuccess && data.ReachabilityError != "" {
		// OSI GUARDRAIL: If DNS failed, we don't even have a target IP.
		// Suppress generic reachability findings if it was clearly a resolution error.
		if !data.DNSSuccess {
			return nil
		}

		findings = append(findings, Finding{
			ID:                 "REACH_FAIL_01",
			Category:           "Reachability",
			Severity:           SeverityHigh,
			Confidence:         1.0,
			Title:              "Target Unreachable",
			Description:        "Could not establish a TCP connection to the target.",
			Evidence:           []string{e.mask(data.ReachabilityError)},
			ProbableCauses:     []string{"Firewall blocking TCP port", "Target service not listening", "ISP or VPN blocking"},
			RecommendedActions: []string{"Verify the target IP and port", "Check local Windows Firewall rules", "Try from a different network"},
		})
	} else if data.ReachabilitySuccess && data.ReachabilityRTT > 300 {
		findings = append(findings, Finding{
			ID:                 "REACH_LATENCY_02",
			Category:           "Reachability",
			Severity:           SeverityHigh,
			Confidence:         0.9,
			Title:              "Very High Latency",
			Description:        fmt.Sprintf("RTT of %.0fms is severely degraded (threshold: 300ms).", data.ReachabilityRTT),
			Evidence:           []string{fmt.Sprintf("Observed RTT: %.1fms", data.ReachabilityRTT)},
			ProbableCauses:     []string{"Congested WAN link", "VPN tunnel overhead", "Geographic distance exceeding expected"},
			RecommendedActions: []string{"Run path trace to identify congested hop", "Check VPN split tunneling configuration"},
		})
	} else if data.ReachabilitySuccess && data.ReachabilityRTT > 100 {
		findings = append(findings, Finding{
			ID:             "REACH_LATENCY_01",
			Category:       "Reachability",
			Severity:       SeverityMedium,
			Confidence:     0.8,
			Title:          "Elevated Latency",
			Description:    fmt.Sprintf("RTT of %.0fms exceeds the 100ms acceptable threshold.", data.ReachabilityRTT),
			Evidence:       []string{fmt.Sprintf("Observed RTT: %.1fms", data.ReachabilityRTT)},
			ProbableCauses: []string{"Geographic distance", "Congested intermediate link", "Wireless interference"},
			RecommendedActions: []string{
				"Run MTR trace to pinpoint the latent hop",
				"Compare against a wired connection",
			},
		})
	}
	return findings
}

func (e *Engine) analyzeDNS(data InputData) []Finding {
	var findings []Finding
	// OSI Layer Validation: If Layer 4 (TCP) succeeded, Layer 3 (DNS) MUST have succeeded.
	// We skip reporting DNS failure if reachability was successful to avoid false positives.
	if !data.DNSSuccess && !data.ReachabilitySuccess {
		findings = append(findings, Finding{
			ID:          "DNS_FAIL_01",
			Category:    "DNS",
			Severity:    SeverityCritical,
			Confidence:  1.0,
			Title:       "DNS Resolution Failed",
			Description: "The hostname could not be resolved. All subsequent tests are unreliable.",
			Evidence:    []string{"DNS query returned no results"},
			ProbableCauses: []string{
				"Incorrect DNS server",
				"No network connectivity",
				"Domain does not exist",
				"ISP/Firewall blocking UDP 53",
			},
			RecommendedActions: []string{
				"Run: nslookup <host>",
				"Try alternate DNS: nslookup <host> 1.1.1.1 and 8.8.8.8",
				"Verify local configuration: ipconfig /all",
				"Check if UDP/TCP 53 is blocked by firewall or ISP",
				"Try from another network (e.g. mobile hotspot) to isolate local issues",
			},
		})
	} else if data.DNSDuration > 300 {
		findings = append(findings, Finding{
			ID:          "DNS_SLOW_01",
			Category:    "DNS",
			Severity:    SeverityMedium,
			Confidence:  0.85,
			Title:       "Slow DNS Resolution",
			Description: fmt.Sprintf("DNS resolution took %.0fms (threshold: 300ms). This will add latency to all connections.", data.DNSDuration),
			Evidence:    []string{fmt.Sprintf("DNS duration: %.1fms", data.DNSDuration)},
			ProbableCauses: []string{
				"DNS server is distant or overloaded",
				"No local DNS cache hit",
				"VPN routing DNS queries to a slow resolver",
			},
			RecommendedActions: []string{"Configure a faster DNS server (e.g. 8.8.8.8)", "Enable local DNS caching"},
		})
	}
	return findings
}

func (e *Engine) analyzeTLS(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If DNS failed, TLS never started. Do not report TLS findings.
	if !data.DNSSuccess || !data.ReachabilitySuccess {
		return nil
	}

	if data.TLSHandshakeTime > 800 {
		findings = append(findings, Finding{
			ID:          "TLS_SLOW_01",
			Category:    "TLS",
			Severity:    SeverityMedium,
			Confidence:  0.8,
			Title:       "Slow TLS Handshake",
			Description: fmt.Sprintf("TLS handshake took %.0fms (threshold: 800ms). Session setup latency is abnormally high.", data.TLSHandshakeTime),
			Evidence:    []string{fmt.Sprintf("Handshake duration: %.1fms", data.TLSHandshakeTime)},
			ProbableCauses: []string{
				"Long certificate chain requiring multiple round trips",
				"OCSP stapling not configured — online validation adds latency",
				"Server-side crypto performance issue",
			},
			RecommendedActions: []string{
				"Check server certificate chain length",
				"Enable OCSP stapling on the server",
				"Verify server CPU is not saturated",
			},
		})
	} else if !data.TLSSuccess && data.ReachabilitySuccess && data.TLSHandshakeTime > 0 {
		// Only report TLS failure if we actually attempted a handshake (TLSHandshakeTime > 0)
		// This avoids reporting TLS failure on plain HTTP (port 80) tests.
		findings = append(findings, Finding{
			ID:                 "TLS_FAIL_01",
			Category:           "TLS",
			Severity:           SeverityHigh,
			Confidence:         0.95,
			Title:              "TLS Handshake Failed",
			Description:        "TCP connection succeeded but TLS negotiation failed. The service may be non-TLS or using an incompatible cipher.",
			Evidence:           []string{"TLS handshake error on established TCP connection"},
			ProbableCauses:     []string{"Certificate expired or self-signed", "Protocol mismatch (TLS version)", "Wrong SNI"},
			RecommendedActions: []string{"Verify certificate validity", "Check SNI configuration", "Test with openssl s_client"},
		})
	}
	return findings
}

func (e *Engine) analyzePath(data InputData) []Finding {
	var findings []Finding
	for i, hop := range data.Hops {
		// Latency spike between adjacent hops
		if i > 0 {
			prev := data.Hops[i-1]
			delta := hop.RTT - prev.RTT
			if delta > 80 {
				sev := SeverityMedium
				conf := 0.80
				if delta > 150 {
					sev = SeverityHigh
					conf = 0.90
				}
				findings = append(findings, Finding{
					ID:          fmt.Sprintf("PATH_SPIKE_%02d", i+1),
					Category:    "Path",
					Severity:    sev,
					Confidence:  conf,
					Title:       fmt.Sprintf("Latency Spike at Hop %d", i+1),
					Description: fmt.Sprintf("RTT jumped +%.0fms between hop %d (%s) and hop %d (%s).", delta, i, e.mask(prev.Host), i+1, e.mask(hop.Host)),
					Evidence: []string{
						fmt.Sprintf("Hop %d RTT: %.1fms", i, prev.RTT),
						fmt.Sprintf("Hop %d RTT: %.1fms", i+1, hop.RTT),
					},
					ProbableCauses:     []string{"Congested inter-provider link", "Bufferbloat", "Policy-based routing change"},
					RecommendedActions: []string{"Validate provider SLA at the identified segment", "Run continuous MTR to confirm stability", "Test at off-peak hours"},
				})
			}
		}

		// Packet loss
		if hop.Loss > 5.0 {
			sev := SeverityMedium
			if hop.Loss > 25.0 {
				sev = SeverityHigh
			}
			if hop.Loss > 60.0 {
				sev = SeverityCritical
			}
			findings = append(findings, Finding{
				ID:                 fmt.Sprintf("PATH_LOSS_%02d", i+1),
				Category:           "Path",
				Severity:           sev,
				Confidence:         0.9,
				Title:              fmt.Sprintf("Packet Loss at Hop %d (%.0f%%)", i+1, hop.Loss),
				Description:        fmt.Sprintf("%.1f%% probe loss observed at hop %d (%s).", hop.Loss, i+1, e.mask(hop.Host)),
				Evidence:           []string{fmt.Sprintf("Loss: %.1f%% at TTL=%d", hop.Loss, hop.TTL)},
				ProbableCauses:     []string{"Link congestion", "ICMP rate-limiting (benign for intermediate hops)", "Faulty interface"},
				RecommendedActions: []string{"Compare loss at next hop — if next hop is clean, this hop is just rate-limiting ICMP", "Contact upstream provider if persistent"},
			})
		}

		// P95 spike on a specific hop
		if hop.P95 > 0 && hop.RTT > 0 && hop.P95 > hop.RTT*3 {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("PATH_JITTER_%02d", i+1),
				Category:    "Path",
				Severity:    SeverityLow,
				Confidence:  0.75,
				Title:       fmt.Sprintf("High Jitter at Hop %d", i+1),
				Description: fmt.Sprintf("P95 RTT (%.0fms) is >3× the average RTT (%.0fms) at hop %d.", hop.P95, hop.RTT, i+1),
				Evidence:    []string{fmt.Sprintf("Avg: %.1fms, P95: %.1fms", hop.RTT, hop.P95)},
				ProbableCauses: []string{
					"Unstable wireless segment",
					"Buffer fluctuation on congested link",
				},
				RecommendedActions: []string{"Run continuous MTR for at least 5 minutes", "Check for wireless interference on client"},
			})
		}
	}
	return findings
}

func (e *Engine) analyzePerformance(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If connectivity failed, performance metrics are invalid.
	if !data.DNSSuccess || !data.ReachabilitySuccess {
		return nil
	}
	if data.ThroughputMbps > 0 {
		if data.ThroughputMbps < 1.0 {
			findings = append(findings, Finding{
				ID:                 "PERF_CRITICAL_01",
				Category:           "Performance",
				Severity:           SeverityCritical,
				Confidence:         0.9,
				Title:              "Critically Low Throughput (<1 Mbps)",
				Description:        fmt.Sprintf("Measured throughput is %.2f Mbps. Essentially unusable for any meaningful transfer.", data.ThroughputMbps),
				Evidence:           []string{fmt.Sprintf("Measured: %.2f Mbps", data.ThroughputMbps)},
				ProbableCauses:     []string{"Severe network congestion", "VPN tunnel bottleneck", "Rate limiting policy", "Client/server OS buffer starvation"},
				RecommendedActions: []string{"Check available bandwidth with a local iperf server", "Disable VPN temporarily to isolate", "Check for background transfers"},
			})
		} else if data.ThroughputMbps < 10.0 {
			findings = append(findings, Finding{
				ID:                 "PERF_LOW_01",
				Category:           "Performance",
				Severity:           SeverityMedium,
				Confidence:         0.75,
				Title:              "Low Throughput (<10 Mbps)",
				Description:        fmt.Sprintf("Throughput of %.2f Mbps is below the 10 Mbps threshold for acceptable performance.", data.ThroughputMbps),
				Evidence:           []string{fmt.Sprintf("Measured: %.2f Mbps", data.ThroughputMbps)},
				ProbableCauses:     []string{"Wi-Fi signal degradation", "Bandwidth throttling", "Background downloads", "TCP window size limitation"},
				RecommendedActions: []string{"Check physical/wireless link speed", "Close other bandwidth-intensive applications", "Try with TCP window scaling enabled"},
			})
		}
	}
	return findings
}

func (e *Engine) analyzeQoS(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If connectivity failed, QoS analysis is invalid.
	if !data.DNSSuccess || !data.ReachabilitySuccess {
		return nil
	}
	// QoS detection: TCP passes but UDP experiences significant loss
	if data.TCPSuccess && data.UDPSuccess && data.UDPLossPct > 15.0 {
		finds := Finding{
			ID:          "QOS_SHAPE_01",
			Category:    "QoS",
			Severity:    SeverityHigh,
			Confidence:  0.88,
			Title:       "UDP Traffic Shaping Detected",
			Description: fmt.Sprintf("TCP connectivity is healthy but UDP shows %.1f%% loss. This asymmetry is characteristic of QoS/traffic shaping policies.", data.UDPLossPct),
			Evidence: []string{
				"TCP: Successful connection established",
				fmt.Sprintf("UDP: %.1f%% packet loss", data.UDPLossPct),
			},
			ProbableCauses: []string{
				"Enterprise firewall or WAN optimizer applying QoS",
				"ISP traffic shaping (common on cellular/VSAT)",
				"VPN provider blocking non-TCP protocols",
			},
			RecommendedActions: []string{
				"Confirm with network admin whether UDP is permitted on this path",
				"Test VoIP and video quality — UDP shaping will degrade real-time traffic",
				"If VPN-related, check VPN protocol settings (UDP vs TCP tunnel)",
			},
		}
		findings = append(findings, finds)
	}
	return findings
}

func (e *Engine) analyzeJitter(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If connectivity failed, jitter analysis is invalid.
	if !data.DNSSuccess || !data.ReachabilitySuccess {
		return nil
	}
	if data.P95RTT > 0 && data.AvgRTT > 0 && data.P95RTT > data.AvgRTT*2.5 {
		findings = append(findings, Finding{
			ID:          "JITTER_01",
			Category:    "Stability",
			Severity:    SeverityMedium,
			Confidence:  0.82,
			Title:       "High End-to-End Jitter",
			Description: fmt.Sprintf("P95 RTT (%.0fms) is %.1f× the average (%.0fms), indicating an unstable path.", data.P95RTT, data.P95RTT/data.AvgRTT, data.AvgRTT),
			Evidence: []string{
				fmt.Sprintf("Average RTT: %.1fms", data.AvgRTT),
				fmt.Sprintf("P95 RTT: %.1fms", data.P95RTT),
				fmt.Sprintf("Ratio: %.1f×", data.P95RTT/data.AvgRTT),
			},
			ProbableCauses: []string{
				"Wireless interference or signal variation",
				"Bufferbloat on an intermediate router",
				"Competing traffic causing queue fluctuation",
			},
			RecommendedActions: []string{
				"Run MTR in continuous mode to track which hop introduces jitter",
				"Switch from Wi-Fi to wired if possible",
				"Check router QoS / queue settings",
			},
		})
	}
	return findings
}

func (e *Engine) analyzeStreams(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If connectivity failed, stream analysis is invalid.
	if !data.DNSSuccess || !data.ReachabilitySuccess {
		return nil
	}
	if len(data.Streams) < 2 {
		return findings
	}
	var maxMbps, minMbps float64
	for i, s := range data.Streams {
		if i == 0 || s.Throughput > maxMbps {
			maxMbps = s.Throughput
		}
		if i == 0 || s.Throughput < minMbps {
			minMbps = s.Throughput
		}
	}
	if minMbps > 0 && maxMbps/minMbps > 3.0 {
		findings = append(findings, Finding{
			ID:          "PERF_STREAM_IMBALANCE_01",
			Category:    "Performance",
			Severity:    SeverityMedium,
			Confidence:  0.78,
			Title:       "Unbalanced Parallel Streams",
			Description: fmt.Sprintf("Max stream (%.1f Mbps) is %.1f× the min stream (%.1f Mbps). This imbalance suggests TCP window or congestion issues.", maxMbps, maxMbps/minMbps, minMbps),
			Evidence: []string{
				fmt.Sprintf("Fastest stream: %.1f Mbps", maxMbps),
				fmt.Sprintf("Slowest stream: %.1f Mbps", minMbps),
				fmt.Sprintf("Imbalance ratio: %.1f×", maxMbps/minMbps),
			},
			ProbableCauses: []string{
				"TCP congestion control competing across streams",
				"Receive window size limiting some streams",
				"ECMP flow hash asymmetry in the network",
			},
			RecommendedActions: []string{
				"Reduce parallel stream count and re-test",
				"Increase TCP receive buffer sizes on client/server",
				"Check if ECMP is active in the path",
			},
		})
	}
	return findings
}

func (e *Engine) analyzeScan(data InputData) []Finding {
	var findings []Finding
	// OSI GUARDRAIL: If resolution failed, scanning is mostly noise or invalid.
	if !data.DNSSuccess {
		return nil
	}

	for _, res := range data.ScanResults {
		if res.Open {
			sev := SeverityInfo
			conf := 0.95
			desc := fmt.Sprintf("Port %d is open and identifying as %s.", res.Port, res.Service)

			// Identify potentially risky ports
			risky := false
			if res.Port == 21 || res.Port == 23 || res.Port == 445 {
				risky = true
				sev = SeverityHigh
				desc += " This protocol is unencrypted or carries high security risk (e.g., Telnet, FTP, SMB)."
			}

			evidence := []string{fmt.Sprintf("Port: %d", res.Port), fmt.Sprintf("Service: %s", res.Service)}
			if res.Banner != "" {
				evidence = append(evidence, fmt.Sprintf("Banner: %s", res.Banner))
			}

			finding := Finding{
				ID:          fmt.Sprintf("SCAN_PORT_%d", res.Port),
				Category:    "Security",
				Severity:    sev,
				Confidence:  conf,
				Title:       fmt.Sprintf("Open Port Detected: %d (%s)", res.Port, res.Service),
				Description: desc,
				Evidence:    evidence,
			}

			if risky {
				finding.ProbableCauses = []string{"Service intentionally exposed", "Misconfigured firewall"}
				finding.RecommendedActions = []string{"Close this port if not needed", "Use a VPN or SSH tunnel instead", "Ensure strong authentication"}
			}

			findings = append(findings, finding)
		}
	}
	return findings
}

func (e *Engine) analyzeDiscovery(data InputData) []Finding {
	var findings []Finding
	if len(data.DiscoveryResults) == 0 {
		return nil
	}

	activeHosts := len(data.DiscoveryResults)
	findings = append(findings, Finding{
		ID:          "NET_DISCOVERY_SUMMARY",
		Category:    "Discovery",
		Severity:    SeverityInfo,
		Confidence:  1.0,
		Title:       fmt.Sprintf("Network Discovery: %d Hosts Active", activeHosts),
		Description: fmt.Sprintf("Scanned subnet and identified %d responsive hosts.", activeHosts),
		Evidence:    []string{fmt.Sprintf("Hosts found: %d", activeHosts)},
	})

	// Check for common patterns across hosts
	smbCount := 0
	for _, host := range data.DiscoveryResults {
		for _, port := range host.ScanResults {
			if port.Open && port.Port == 445 {
				smbCount++
			}
		}
	}

	if smbCount > 1 {
		findings = append(findings, Finding{
			ID:                 "NET_SEGMENTATION_RISK_01",
			Category:           "Security",
			Severity:           SeverityHigh,
			Confidence:         0.9,
			Title:              "Widespread SMB Exposure",
			Description:        fmt.Sprintf("SMB (port 445) detected on %d different hosts. This increases the risk of lateral movement and ransomware propagation in the local segment.", smbCount),
			Evidence:           []string{fmt.Sprintf("Instances of SMB: %d", smbCount)},
			ProbableCauses:     []string{"Flat network architecture", "Missing host isolation"},
			RecommendedActions: []string{"Implement VLAN segmentation", "Restrict SMB traffic to authorized file servers only"},
		})
	}

	return findings
}

// ─── Privacy Mask ─────────────────────────────────────────────────────────────

var ipRegex = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var hostRegex = regexp.MustCompile(`\b[a-zA-Z0-9-]+\.[a-zA-Z]{2,}\b`)

func (e *Engine) mask(s string) string {
	if !e.PrivacyMode {
		return s
	}
	s = ipRegex.ReplaceAllString(s, "[masked-ip]")
	s = hostRegex.ReplaceAllString(s, "[masked-host]")
	return s
}

// ─── Narrative Engine ─────────────────────────────────────────────────────────

// NarrativeEngine generates human-readable text from structured findings.
type NarrativeEngine struct {
	privacyMode bool
}

// GenerateSummary produces an executive summary paragraph.
func (n *NarrativeEngine) GenerateSummary(findings []Finding, baseFailure bool) string {
	if len(findings) == 0 {
		if baseFailure {
			return "Network path health is not evaluated because the foundation connectivity (DNS/Reachability) failed."
		}
		return "All diagnostic checks passed. No anomalies were detected."
	}

	critical := filterBySeverity(findings, SeverityCritical)
	high := filterBySeverity(findings, SeverityHigh)
	medium := filterBySeverity(findings, SeverityMedium)

	var parts []string

	// Check for DNS failure specifically for the summary
	dnsFailed := false
	for _, f := range findings {
		if f.ID == "DNS_FAIL_01" {
			dnsFailed = true
			break
		}
	}

	if dnsFailed {
		parts = append(parts, "Network path health is not evaluated because DNS resolution failed. Connectivity beyond the local resolver cannot be assessed due to DNS timeout.")
	} else {
		if len(critical) > 0 {
			titles := joinTitles(critical)
			parts = append(parts, fmt.Sprintf("**Critical issues** detected: %s.", titles))
		}
		if len(high) > 0 {
			titles := joinTitles(high)
			parts = append(parts, fmt.Sprintf("High-severity findings: %s.", titles))
		}
		if len(medium) > 0 {
			titles := joinTitles(medium)
			parts = append(parts, fmt.Sprintf("Medium-severity observations: %s.", titles))
		}
	}

	if len(parts) == 0 {
		if baseFailure {
			return "Connectivity beyond the base layer cannot be assessed due to a critical transport failure."
		}
		return "Only informational or low-severity findings were detected. The network path appears functional for the evaluated metrics."
	}

	summary := strings.Join(parts, " ")

	// Highest confidence finding drives the closing sentence
	top := topFinding(findings)
	if top != nil && top.Confidence >= 0.85 {
		summary += fmt.Sprintf(
			" The most significant finding is **%s** (confidence: %.0f%%), based on: %s.",
			top.Title, top.Confidence*100, strings.Join(top.Evidence, "; "),
		)
	} else if top != nil {
		summary += fmt.Sprintf(
			" The highest-confidence finding is **%s** (confidence: %.0f%%). Further investigation is recommended to confirm.",
			top.Title, top.Confidence*100,
		)
	}

	summary += "\n\n*This analysis is generated automatically by the DiagAll offline AI engine. It does not replace expert validation.*"
	return summary
}

// GenerateNextSteps returns an ordered list of recommended diagnostic steps.
func (n *NarrativeEngine) GenerateNextSteps(findings []Finding, baseFailure bool) []string {
	seen := map[string]bool{}
	var steps []string

	// Dedup and collect all recommended actions, prioritised by severity
	for _, sev := range []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow} {
		for _, f := range findings {
			if f.Severity != sev {
				continue
			}
			for _, a := range f.RecommendedActions {
				if !seen[a] {
					seen[a] = true
					steps = append(steps, a)
				}
			}
		}
	}

	if len(steps) == 0 {
		if baseFailure {
			return []string{"Immediate investigation of DNS or Reachability is required."}
		}
		return []string{"No action required — all metrics are within normal thresholds."}
	}
	return steps
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func filterBySeverity(findings []Finding, sev string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

func joinTitles(findings []Finding) string {
	var titles []string
	for _, f := range findings {
		titles = append(titles, f.Title)
	}
	return strings.Join(titles, ", ")
}

func topFinding(findings []Finding) *Finding {
	order := map[string]int{SeverityCritical: 4, SeverityHigh: 3, SeverityMedium: 2, SeverityLow: 1, SeverityInfo: 0}
	var top *Finding
	for i := range findings {
		f := &findings[i]
		if top == nil {
			top = f
			continue
		}
		if order[f.Severity] > order[top.Severity] {
			top = f
		} else if order[f.Severity] == order[top.Severity] && f.Confidence > top.Confidence {
			top = f
		}
	}
	return top
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
