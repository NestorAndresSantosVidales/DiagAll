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

## 🛠 Quick Start & CLI Command Reference

To use DiagAll via the CLI, you interact with specific subcommands.

### `help`
Prints the help menu with all available commands.
```bash
diagall help
```

---

### `reach`
Tests TCP or UDP reachability to a specific target host and port.
- **Syntax:** `diagall reach [--proto tcp|udp] [--timeout 2s] <host:port>`
- **Combinations:**
  - `diagall reach google.com:443` (Defaults to TCP).
  - `diagall reach -proto udp -timeout 5s 192.168.1.1:53`

> **Expected Output:**
> ```text
> TCP Reach google.com:443: Success, RTT=14.5ms
> 
> --- 🤖 Expert Analysis ---
> The target is reachable on the specified port. Connection established successfully in 14.5ms.
> --------------------------
> ```

---

### `dns`
Resolves a domain name to its IP addresses.
- **Syntax:** `diagall dns <host>`

> **Expected Output:**
> ```text
> DNS Resolve github.com:
>         IPs: [140.82.113.3]
>         Duration: 34.2ms
>         Error: <nil>
>         
> --- 🤖 Expert Analysis ---
> DNS resolution successful.
> --------------------------
> ```

---

### `tls`
Performs a TLS handshake to check certificate validity and connection security.
- **Syntax:** `diagall tls <host:port>`

> **Expected Output:**
> ```text
> TLS Check example.com:443:
>         Version: 304
>         Cipher: 1301
>         Handshake: 45.1ms
>         Success: true
>         Error: <nil>
> ```

---

### `trace`
Traces the route to the target over TCP (useful for bypassing ICMP blocking).
- **Syntax:** `diagall trace <host:port>`

> **Expected Output:**
> ```text
> Tracing route to 8.8.8.8:443 over TCP...
> ..........
> 1       4.2ms   192.168.1.1     Success
> 2       12.1ms  10.0.0.1        Success
> 3       14.5ms  8.8.8.8         Success
> 
> --- 🤖 Expert Analysis ---
> Route traced successfully in 3 hops with no packet loss detected.
> --------------------------
> ```

---

### `perf`
Measures bandwidth throughput. Can run in client or server mode for TCP or UDP.
- **Syntax:** `diagall perf <tcp|udp> <client|server> [args]`
- **Combinations:**
  - **Server (Listen):** `diagall perf tcp server 9090`
  - **Client (Test):** `diagall perf tcp client 192.168.1.50:9090`

> **Expected Client Output:**
> ```text
> TCP Performance: 112.45 MB/s
> ```

---

### `scan`
Targeted port scanning for service discovery.
- **Syntax:** `diagall scan <host> [--ports port1,port2...] [--timeout 500ms]`
- **Combinations:**
  - `diagall scan 10.0.0.5` (Scans default popular ports).
  - `diagall scan 10.0.0.5 -ports 22,80,443,8080 -timeout 1s`

> **Expected Output:**
> ```text
> 🔍 Scanning 4 ports on 10.0.0.5...
>   [+] Port 22: OPEN (ssh)
>       Banner: SSH-2.0-OpenSSH_8.2p1 Ubuntu-4ubuntu0.5
>   [+] Port 80: OPEN (http)
> Scan complete. Found 2 open ports.
> ```

---

### `netscan`
Discovers active hosts on a local subnet and checks specific ports.
- **Syntax:** `diagall netscan <cidr> [--ports port1,port2...] [--timeout 300ms]`
- **Combinations:**
  - `diagall netscan 192.168.1.0/24 -ports 22,3389`

> **Expected Output:**
> ```text
> 🌐 Starting network discovery on 192.168.1.0/24...
> 
> [+] Host Found: 192.168.1.10
>     - Port 22: OPEN (ssh)
> 
> [+] Host Found: 192.168.1.15
>     - Port 3389: OPEN (rdp)
> 
> Discovery complete. Found 2 active hosts.
> ```

---

### `profile`
Runs predefined automated troubleshooting profiles and generates an HTML report.
- **Syntax:** `diagall profile <profile_name> <target>`
- **Combinations:**
  - `diagall profile wan google.com`

> **Expected Output:**
> *Executes a batch of Reach, DNS, and Trace commands and outputs an HTML file (e.g., `report_session_1771384719.html`) in the current directory.*

---

### `ask`
Directly engages the embedded Mistral AI Expert to perform guided investigations.
- **Syntax:** `diagall ask "<question or scenario>"`
- **Combinations:**
  - `diagall ask "Why can't I reach 8.8.8.8?"`

> **Expected Output:**
> ```text
> 🤖 AI Expert is initiating a guided investigation for: Why can't I reach 8.8.8.8?
> -------------------------------------------
> I will start by tracing the route to 8.8.8.8 on port 443 to identify where the connection is dropping...
> 
> 🚀 AI requested execution: trace 8.8.8.8
> Tracing... Done.
> 📊 Result: Trace 8.8.8.8: 15 hops reached
> 
> 🤔 Final AI Interpretation:
> -------------------------------------------
> The trace completed 15 hops but failed to reach the final destination. This indicates a routing loop or a strict firewall drop deep in the ISP's network infrastructure.
> ```

---

### `ui`
Starts the local web server to access the GUI.
- **Syntax:** `diagall ui`

> **Expected Output:**
> ```text
> Please open http://localhost:8080 in your browser
> Server started on :8080
> ```
