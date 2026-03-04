package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"diagall/internal/diagnosis"
	"diagall/internal/engine"
	"diagall/internal/probes"
	"diagall/internal/report"
	"diagall/internal/server"
)

func main() {
	diagEngine := diagnosis.NewEngine()

	reachCmd := flag.NewFlagSet("reach", flag.ExitOnError)
	reachProto := reachCmd.String("proto", "tcp", "Protocol (tcp, udp)")
	reachTimeout := reachCmd.Duration("timeout", 2*time.Second, "Timeout for connection")

	if len(os.Args) < 2 {
		fmt.Println("expected subcommand: reach, dns, tls, trace, perf, scan, netscan, ask")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "reach":
		reachCmd.Parse(os.Args[2:])
		args := reachCmd.Args()
		if len(args) < 1 {
			fmt.Println("expected target host:port")
			os.Exit(1)
		}
		target := args[0]

		host, port, err := net.SplitHostPort(target)
		if err != nil {
			fmt.Printf("Invalid target format: %v\n", err)
			os.Exit(1)
		}
		var p int
		_, err = fmt.Sscanf(port, "%d", &p)
		if err != nil {
			fmt.Printf("Invalid port: %v\n", err)
			os.Exit(1)
		}

		if *reachProto == "tcp" {
			result := probes.ReachTCP(host, p, *reachTimeout)
			if result.Success {
				fmt.Printf("TCP Reach %s: Success, RTT=%v\n", result.Target, result.RTT)
			} else {
				fmt.Printf("TCP Reach %s: Failed, Error=%v, RTT=%v\n", result.Target, result.Error, result.RTT)
			}
			printAIAnalysis(diagEngine, diagnosis.InputData{
				ReachabilitySuccess: result.Success,
				ReachabilityRTT:     float64(result.RTT.Milliseconds()),
				ReachabilityError:   fmt.Sprintf("%v", result.Error),
				TCPSuccess:          result.Success,
			})
		} else if *reachProto == "udp" {
			result := probes.ReachUDP(host, p, *reachTimeout, nil)
			if result.Success {
				fmt.Printf("UDP Reach %s: Success, RTT=%v\n", result.Target, result.RTT)
			} else {
				fmt.Printf("UDP Reach %s: Failed, Error=%v (Note: UDP requires an echo service)\n", result.Target, result.Error)
			}
			printAIAnalysis(diagEngine, diagnosis.InputData{
				ReachabilitySuccess: result.Success,
				ReachabilityRTT:     float64(result.RTT.Milliseconds()),
				ReachabilityError:   fmt.Sprintf("%v", result.Error),
				UDPSuccess:          result.Success,
			})
		} else {
			fmt.Println("Unknown protocol. Use tcp or udp.")
		}

	case "dns":
		if len(os.Args) < 3 {
			fmt.Println("expected target host")
			os.Exit(1)
		}
		target := os.Args[2]
		result := probes.Resolve(target)
		fmt.Printf("DNS Resolve %s:\n\tIPs: %v\n\tDuration: %v\n\tError: %v\n", result.Target, result.ResolvedIPs, result.Duration, result.Error)
		printAIAnalysis(diagEngine, diagnosis.InputData{
			DNSSuccess:  result.Error == nil,
			DNSDuration: float64(result.Duration.Milliseconds()),
		})

	case "tls":
		if len(os.Args) < 3 {
			fmt.Println("expected target host:port")
			os.Exit(1)
		}
		target := os.Args[2]

		host, port, err := net.SplitHostPort(target)
		if err != nil {
			fmt.Printf("Invalid target format: %v\n", err)
			os.Exit(1)
		}
		var p int
		_, err = fmt.Sscanf(port, "%d", &p)
		if err != nil {
			fmt.Printf("Invalid port: %v\n", err)
			os.Exit(1)
		}

		result := probes.CheckTLS(host, p, 5*time.Second)
		fmt.Printf("TLS Check %s:\n\tVersion: %x\n\tCipher: %x\n\tHandshake: %v\n\tSuccess: %v\n\tError: %v\n", result.Target, result.Version, result.CipherSuite, result.HandshakeTime, result.Success, result.Error)
		printAIAnalysis(diagEngine, diagnosis.InputData{
			ReachabilitySuccess: true, // If we reached TLS, TCP worked
			TLSSuccess:          result.Success,
			TLSHandshakeTime:    float64(result.HandshakeTime.Milliseconds()),
		})

	case "trace":
		if len(os.Args) < 3 {
			fmt.Println("expected target host:port")
			os.Exit(1)
		}
		target := os.Args[2]
		host, port, err := net.SplitHostPort(target)
		if err != nil {
			fmt.Printf("Invalid target format: %v\n", err)
			os.Exit(1)
		}
		var p int
		_, err = fmt.Sscanf(port, "%d", &p)
		if err != nil {
			fmt.Printf("Invalid port: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Tracing route to %s over TCP...\n", target)
		hops, err := probes.TraceTCP(host, p, 30, 2*time.Second, func(h probes.TraceHop) {
			// Optional: print real-time in CLI too if desired, but for now we rely on the final loop for CLI format consistency.
			// Or we can just print dots?
			fmt.Print(".")
		})
		fmt.Println("") // Newline after dots

		if err != nil {
			fmt.Printf("Trace failed: %v\n", err)
			os.Exit(1)
		}

		for _, hop := range hops {
			status := "Success"
			if hop.Timeout {
				status = "Timeout"
			}
			if hop.Error != nil {
				status = fmt.Sprintf("Error: %v", hop.Error)
			}
			fmt.Printf("%d\t%v\t%s\t%s\n", hop.TTL, hop.RTT, hop.Host, status)
			if hop.Final {
				break
			}
		}

		// AI Analysis for Trace
		var aiHops []diagnosis.HopData
		for _, h := range hops {
			aiHops = append(aiHops, diagnosis.HopData{
				TTL:  h.TTL,
				Host: h.Host,
				RTT:  float64(h.RTT.Milliseconds()),
				Loss: 0, // TraceTCP doesn't provide loss per hop in this simple CLI implementation
			})
		}
		printAIAnalysis(diagEngine, diagnosis.InputData{Hops: aiHops})

	case "perf":
		// perf tcp client host:port
		// perf tcp server port
		// For now simple: perf mode [args]
		// Spec says perf tcp and perf udp.
		// Let's do: perf tcp server <port> / perf tcp client <host:port>
		if len(os.Args) < 4 {
			fmt.Println("expected: perf <proto> <mode> [args]")
			os.Exit(1)
		}
		proto := os.Args[2]
		mode := os.Args[3]

		if proto == "tcp" {
			if mode == "server" {
				if len(os.Args) < 5 {
					fmt.Println("expected port")
					os.Exit(1)
				}
				var port int
				fmt.Sscanf(os.Args[4], "%d", &port)
				probes.StartTCPServer(port, 10*time.Second)
			} else if mode == "client" {
				if len(os.Args) < 5 {
					fmt.Println("expected target host:port")
					os.Exit(1)
				}
				target := os.Args[4]
				host, portSt, _ := net.SplitHostPort(target)
				var port int
				fmt.Sscanf(portSt, "%d", &port)

				res := probes.RunTCPClient(host, port, 1, 5*time.Second, nil)
				fmt.Printf("TCP Performance: %.2f MB/s\n", res.ThroughputBps/1024/1024)
				printAIAnalysis(diagEngine, diagnosis.InputData{
					ThroughputMbps: res.ThroughputBps / 1024 / 1024 * 8, // Bps to Mbps
					TCPSuccess:     true,
				})
			}
		} else if proto == "udp" {
			if mode == "server" {
				if len(os.Args) < 5 {
					fmt.Println("expected port")
					os.Exit(1)
				}
				var port int
				fmt.Sscanf(os.Args[4], "%d", &port)
				probes.StartUDPServer(port, 10*time.Second)
			} else if mode == "client" {
				if len(os.Args) < 5 {
					fmt.Println("expected target host:port")
					os.Exit(1)
				}
				target := os.Args[4]
				host, portSt, _ := net.SplitHostPort(target)
				var port int
				fmt.Sscanf(portSt, "%d", &port)

				probes.RunUDPClient(host, port, 5*time.Second)
			}
		} else {
			fmt.Println("Only tcp and udp supported in perf")
		}

	case "profile":
		if len(os.Args) < 4 {
			fmt.Println("expected profile name and target (e.g. wan google.com)")
			os.Exit(1)
		}
		profileName := os.Args[2]
		target := os.Args[3]
		session := engine.RunProfile(profileName, target, nil)

		err := report.GenerateReport(session)
		if err != nil {
			fmt.Printf("Failed to generate report: %v\n", err)
		}

	case "complete":
		// Stub for LLM autocomplete
		// diagall complete "<partial_command>"
		if len(os.Args) > 2 {
			input := os.Args[2]
			// Simple static suggestions for MVP
			if input == "re" {
				fmt.Println("reach")
			}
			if input == "tr" {
				fmt.Println("trace")
			}
			if input == "pro" {
				fmt.Println("profile")
			}
			// In future this would call local LLM
		}

	case "help":
		fmt.Println("DiagAll - Network Diagnostics Tool")
		fmt.Println("Commands:")
		fmt.Println("  reach <tcp|udp> <host:port>   - Test reachability")
		fmt.Println("  dns <host>                    - Resolve DNS")
		fmt.Println("  tls <host:port>               - Check TLS handshake")
		fmt.Println("  trace <host:port>             - Trace path (TCP)")
		fmt.Println("  perf <tcp|udp> <mode> [args]  - Performance test")
		fmt.Println("  scan <host> [--ports 80,443]  - Port scan & service ID")
		fmt.Println("  netscan <cidr> [--ports 80]   - Network-wide discovery")
		fmt.Println("  profile <wan|vpn> <target>    - Run test profile")
		fmt.Println("  complete <input>              - Autocomplete stub")
		fmt.Println("  ask <question>                - Direct AI Expert consultation")

	case "ui":
		port := 8080
		// Open browser (best effort)
		go func() {
			time.Sleep(500 * time.Millisecond)
			fmt.Println("Please open http://localhost:8080 in your browser")
		}()
		server.StartServer(port)

	case "ask":
		runAsk(os.Args[2:])
	case "scan":
		runScan(os.Args[2:], diagEngine)
	case "netscan":
		runNetScan(os.Args[2:], diagEngine)
	default:
		fmt.Println("expected subcommand: reach, dns, tls, trace, perf, profile, scan, netscan, ask or help")
		os.Exit(1)
	}
}

func runAsk(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: diagall ask <your question or scenario>")
		return
	}
	input := strings.Join(args, " ")

	eng := diagnosis.NewEngine()
	if eng.AIExpert == nil {
		fmt.Println("AI Expert not initialized. Ensure models/llama-3.2-1b.gguf exists.")
		return
	}

	fmt.Printf("🤖 AI Expert is initiating a guided investigation for: %s\n", input)
	fmt.Println("-------------------------------------------")

	ctx := context.Background()
	narrative, actions, err := eng.AIExpert.AnalyzeGuided(ctx, input)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(narrative)

	if len(actions) == 0 {
		return
	}

	var resultsSummary []string
	for _, action := range actions {
		fmt.Printf("\n🚀 AI requested execution: %s %s\n", action.Command, action.Target)

		var resultStr string
		switch action.Command {
		case "dns":
			res := probes.Resolve(action.Target)
			resultStr = fmt.Sprintf("DNS %s: IPs=%v, Err=%v", action.Target, res.ResolvedIPs, res.Error)
		case "reach":
			res := probes.ReachTCP(action.Target, 443, 2*time.Second)
			resultStr = fmt.Sprintf("Reach %s:443: Success=%v, RTT=%v", action.Target, res.Success, res.RTT)
		case "trace":
			fmt.Print("Tracing...")
			hops, _ := probes.TraceTCP(action.Target, 443, 10, 1*time.Second, nil)
			fmt.Println(" Done.")
			resultStr = fmt.Sprintf("Trace %s: %d hops reached", action.Target, len(hops))
		default:
			resultStr = fmt.Sprintf("Command %s not implemented for CLI guided mode yet", action.Command)
		}

		fmt.Printf("📊 Result: %s\n", resultStr)
		resultsSummary = append(resultsSummary, resultStr)
	}

	fmt.Println("\n🤔 Final AI Interpretation:")
	fmt.Println("-------------------------------------------")
	followUp := fmt.Sprintf("The results of my requested actions are:\n%s\n\nBased on this, what is your final diagnosis?", strings.Join(resultsSummary, "\n"))
	finalNarrative, _, _ := eng.AIExpert.AnalyzeGuided(ctx, followUp)
	fmt.Println(finalNarrative)
}

func printAIAnalysis(eng *diagnosis.Engine, data diagnosis.InputData) {
	fmt.Println("\n--- 🤖 Expert Analysis ---")
	result := eng.Analyze(data)
	fmt.Println(result.Summary)

	if len(result.Findings) > 0 {
		fmt.Println("\nRecommended Actions:")
		for _, step := range result.NextSteps {
			fmt.Printf("- %s\n", step)
		}
	}
	fmt.Println("--------------------------")
}

func runScan(args []string, eng *diagnosis.Engine) {
	scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
	portsStr := scanCmd.String("ports", "21,22,23,25,53,80,110,443,445,3306,3389,5432,8080", "Comma-separated list of ports to scan")
	scanTimeout := scanCmd.Duration("timeout", 500*time.Millisecond, "Timeout per port")

	if len(args) < 1 {
		fmt.Println("Usage: diagall scan <host> [--ports 80,443] [--timeout 500ms]")
		return
	}

	scanCmd.Parse(args[1:])
	target := args[0]

	var ports []int
	for _, pStr := range strings.Split(*portsStr, ",") {
		var p int
		fmt.Sscanf(pStr, "%d", &p)
		if p > 0 {
			ports = append(ports, p)
		}
	}

	fmt.Printf("🔍 Scanning %d ports on %s...\n", len(ports), target)
	results := probes.PortScan(target, ports, *scanTimeout)

	var diagResults []diagnosis.ScanResult
	openPorts := 0
	for _, r := range results {
		if r.Open {
			fmt.Printf("  [+] Port %d: OPEN (%s)\n", r.Port, r.Service)
			if r.Banner != "" {
				fmt.Printf("      Banner: %s\n", r.Banner)
			}
			openPorts++
		}
		diagResults = append(diagResults, diagnosis.ScanResult{
			Port:    r.Port,
			Open:    r.Open,
			Service: r.Service,
			Banner:  r.Banner,
		})
	}
	fmt.Printf("Scan complete. Found %d open ports.\n", openPorts)

	// AI Analysis
	data := diagnosis.InputData{
		DNSSuccess:  true, // Assume DNS worked if we have a target or we'll gate it in analyzeScan
		ScanResults: diagResults,
	}
	printAIAnalysis(eng, data)
}

func runNetScan(args []string, eng *diagnosis.Engine) {
	netCmd := flag.NewFlagSet("netscan", flag.ExitOnError)
	portsStr := netCmd.String("ports", "80,443,22,445,3389", "Comma-separated list of ports to scan on active hosts")
	scanTimeout := netCmd.Duration("timeout", 300*time.Millisecond, "Timeout per port check")

	if len(args) < 1 {
		fmt.Println("Usage: diagall netscan <cidr> [--ports 80,443] [--timeout 300ms]")
		return
	}

	netCmd.Parse(args[1:])
	cidr := args[0]

	var ports []int
	for _, pStr := range strings.Split(*portsStr, ",") {
		var p int
		fmt.Sscanf(pStr, "%d", &p)
		if p > 0 {
			ports = append(ports, p)
		}
	}

	fmt.Printf("🌐 Starting network discovery on %s...\n", cidr)
	results, err := probes.DiscoverHosts(cidr, ports, *scanTimeout)
	if err != nil {
		fmt.Printf("Discovery failed: %v\n", err)
		return
	}

	var diagDiscovery []diagnosis.HostResult
	for _, host := range results {
		fmt.Printf("\n[+] Host Found: %s\n", host.IP)
		var diagScan []diagnosis.ScanResult
		for _, r := range host.ScanResults {
			if r.Open {
				fmt.Printf("    - Port %d: OPEN (%s)\n", r.Port, r.Service)
				if r.Banner != "" {
					fmt.Printf("      Banner: %s\n", r.Banner)
				}
			}
			diagScan = append(diagScan, diagnosis.ScanResult{
				Port:    r.Port,
				Open:    r.Open,
				Service: r.Service,
				Banner:  r.Banner,
			})
		}
		diagDiscovery = append(diagDiscovery, diagnosis.HostResult{
			IP:          host.IP,
			ScanResults: diagScan,
		})
	}
	fmt.Printf("\nDiscovery complete. Found %d active hosts.\n", len(results))

	// AI Analysis
	data := diagnosis.InputData{
		DNSSuccess:       true,
		DiscoveryResults: diagDiscovery,
	}
	printAIAnalysis(eng, data)
}
