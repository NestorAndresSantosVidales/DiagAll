package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	knowledgeDir := "knowledge"
	_ = os.MkdirAll(knowledgeDir, 0755)

	cloudProviders := []string{"AWS", "Azure", "GCP", "OCI"}
	components := []string{"Security Groups", "NACLs", "VPC Peering", "NAT Gateways", "VPN Gateways", "Load Balancers", "Direct Connect", "FastConnect", "Route Tables", "DNS Zones"}
	scenarios := []string{"Connectivity Timeout", "Intermittent Packet Loss", "Latency Spike", "Throughput Bottleneck", "DNS Resolution Failure", "BGP Flapping", "MTU Mismatch", "Authentication Failure"}
	causes := []string{"Misconfigured rules", "Overlapping CIDRs", "Exceeded service limits", "Provider-side regional issues", "Misaligned MTU values", "Hardware degradation", "Incorrect route priorities"}
	fixes := []string{"Review security rules", "Verify routing tables", "Increase bandwidth capacity", "Adjust MTU to 1460", "Re-validate BGP MD5 hash", "Flush DNS cache", "Switch to redundant link"}

	count := 0
	targetCases := 10500 // Aim for slightly over 10k

	f, _ := os.Create(filepath.Join(knowledgeDir, "kb_massive_expert.txt"))
	defer f.Close()

	for i := 1; i <= targetCases; i++ {
		provider := cloudProviders[i%len(cloudProviders)]
		comp := components[i%len(components)]
		scenario := scenarios[i%len(scenarios)]
		cause := causes[i%len(causes)]
		fix := fixes[i%len(fixes)]

		text := fmt.Sprintf("Case ID: NET-%06d\nProvider: %s\nComponent: %s\nScenario: %s\nRoot Cause: %s\nResolution: %s\n\n",
			i, provider, comp, scenario, cause, fix)

		_, _ = f.WriteString(text)
		count++
	}

	fmt.Printf("Generated %d cases in knowledge/kb_massive_expert.txt\n", count)
}
