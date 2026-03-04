# 🌐 DiagAll – The AI-Powered Network Diagnostics Tool

**DiagAll** is a comprehensive, offline, and AI-driven network diagnostics application. Designed to operate completely without internet dependencies and without requiring administrator privileges, it provides an all-in-one toolkit for network engineers, system administrators, and developers to troubleshoot network issues seamlessly.

A core distinction of this project is that its development and internal analysis engine were heavily powered by **Mistral AI**. By leveraging the capabilities of advanced local language models (LLMs), DiagAll not only performs raw network tests but also interprets the results just like a human network expert would.

---

## ✨ Key Features

DiagAll packs a wide array of network diagnostic capabilities into a single binary, accessible via both a CLI and an intuitive Web UI.

1. **Reachability Testing (`reach`)**
   - **TCP & UDP Probes:** Checks if a specific host and port are accessible over TCP or UDP.
   - **Metrics:** Calculates precise Round Trip Time (RTT).
   - **AI Context:** AI automatically determines if connection failures are due to routing, firewall rules, or service availability.

2. **DNS Resolution (`dns`)**
   - **Fast Lookups:** Resolves domain names to IP addresses instantly.
   - **Latency Tracking:** Measures the exact duration of the DNS query.

3. **TLS Handshake Analysis (`tls`)**
   - **Security Checks:** Verifies TLS certificates, versions, and negotiated cipher suites.
   - **Performance:** Measures handshake time to diagnose slow secure connections.

4. **TCP Route Tracing (`trace`)**
   - **Path Discovery:** Discovers the path packets take to reach a destination over TCP (bypassing common ICMP blocks).
   - **Hop Analysis:** Analyzes TTL, RTT for each hop, and identifies where packets are being dropped.

5. **Performance Testing (`perf`)**
   - **Throughput Measurement:** Runs a localized speed test for TCP and UDP to measure available bandwidth (MB/s).
   - **Client/Server Mode:** Works in both client and server modes to test links between two DiagAll instances.

6. **Targeted Port Scanning (`scan`)**
   - **Service Discovery:** Scans common or custom ports on a single host.
   - **Banner Grabbing:** Attempts to read the service banner (e.g., SSH, HTTP server versions) to identify what is running behind the open port.

7. **Network-Wide Discovery (`netscan`)**
   - **Host Discovery:** Scans an entire CIDR subnet (e.g., `192.168.1.0/24`) to find active hosts.
   - **Fast Port Probing:** Quickly identifies key open ports across all discovered devices in the local network.

8. **AI Expert Guided Investigations (`ask`)**
   - **Interactive Diagnostics:** You can ask the AI questions like: *"Why can't I reach 8.8.8.8?"*. 
   - **Autonomous Probing:** The AI engine will proactively run `dns`, `reach`, and `trace` commands to gather data, analyze the output, and provide a final diagnosis in plain English.

9. **Rich Graphical Web UI (`ui`)**
   - **Tabbed Interface:** A 7-tab graphical dashboard accessible via a local web server displaying live charts, command outputs, and AI narratives in real time.

---

## ⚙️ How It Works & The Role of Mistral AI

DiagAll is built in Go for maximum performance and cross-platform compatibility. It is fundamentally divided into three core engines:
1. **The Probe Engine:** Handles the low-level network packets and timings.
2. **The Server & UI:** Provides the interactive web dashboard.
3. **The Diagnosis Engine:** The intelligent brain of the tool.

### Developed and Powered by Mistral AI
The conception, logic structuring, and code generation for this project were heavily assisted by **Mistral AI**. Furthermore, DiagAll integrates local AI modeling for its runtime analysis:
- **Offline Intelligence:** DiagAll uses a local `.gguf` model (compatible with Mistral-based architectures) to analyze the test results inside the `diagnosis` engine.
- **Narrative Generation:** Instead of just outputting `Timeout on Hop 4`, the AI interprets the data and outputs: *"Packet loss at hop 4 indicates a potential router misconfiguration or firewall drop midway through the ISP network."*

---

## 📈 Benefits and Scope

- **100% Offline Capability:** In environments with strict security policies, air-gapped networks, or total internet outages, DiagAll still provides full diagnostic and AI capabilities locally.
- **Zero Privileges Required:** Unlike traditional tools that require `sudo` or Administrator rights for ICMP or raw sockets, DiagAll uses unprivileged techniques (like TCP Connect scanning and unprivileged tracing) to work out-of-the-box for any user.
- **Democratized Network Engineering:** You don't need to be a CCNA/CCNP to understand the results. The AI translates complex network behavior into actionable insights.
- **All-in-One Binary:** No need to install separate tools like `nmap`, `traceroute`, `ping`, `iperf`, and `nslookup`. DiagAll replaces them all.

---

## 🚀 Upcoming Updates (Roadmap)

The project is continuously evolving. Here is what is planned for the near future:

1. **Enhanced Protocol Support:**
   - Deep inspection for HTTP/2 and HTTP/3 (QUIC) performance.
   - BGP route analysis integration.
2. **Automated Remediation Scripts:** 
   - Not only will the AI diagnose the problem, but it will also generate the CLI commands (e.g., `iptables` or PowerShell `Netsh` rules) needed to fix the issue.
3. **Historical Trending:**
   - A localized SQLite database will store past scans and performance tests, allowing the UI to show degradation over days or weeks.
4. **Packet Capture (PCAP) Analysis Engine:**
   - Ability to ingest `.pcap` files and have the AI Expert parse and identify malicious traffic, broadcast storms, or TCP retransmissions offline.
5. **Cross-Platform Installers:**
   - Packaging as a simple MSI for Windows, DMG for macOS, and Debian package for native OS integration and auto-updating.

---

## 🛠 Quick Start

To use DiagAll via the CLI:

```bash
# Print help menu
diagall help

# Run a port scan
diagall scan 192.168.1.1 --ports 22,80,443

# Ask the AI expert to diagnose a problem
diagall ask "Why is my connection to github.com so slow?"

# Start the Web UI
diagall ui
```
