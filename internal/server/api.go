package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"diagall/internal/diagnosis"
	"diagall/internal/engine"
	"diagall/internal/probes"
)

// Action constants
const (
	ActionRunQuick         = "run_quick"
	ActionRunQuickFull     = "run_quick_full"
	ActionRunMTR           = "run_mtr"
	ActionRunMTRContinuous = "run_mtr_continuous"
	ActionRunReach         = "run_reach"
	ActionRunReachExt      = "run_reach_extended"
	ActionRunPerf          = "run_perf"
	ActionRunPerfExt       = "run_perf_extended"
	ActionRunProfile       = "run_profile"
	ActionAnalyzeSession   = "analyze_session"
	ActionListSessions     = "list_sessions"
	ActionGetReport        = "get_report"
	ActionGetSettings      = "get_settings"
	ActionSaveSettings     = "save_settings"
	ActionStop             = "stop"
	ActionIngestKnowledge  = "ingest_knowledge"
	ActionConsultExpert    = "consult_expert"
	ActionAIGuide          = "ai_guide"
)

// Message from the WebSocket client.
type Message struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// TargetPayload is the rich payload for all test actions.
type TargetPayload struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp" or "udp"
	SNI      string `json:"sni"`
	// Reachability options
	Attempts int     `json:"attempts"`
	Timeout  float64 `json:"timeout_ms"`
	Interval float64 `json:"interval_ms"`
	// Performance options
	Streams   int     `json:"streams"`
	Duration  float64 `json:"duration_s"`
	WarmupSec float64 `json:"warmup_s"`
	Direction string  `json:"direction"` // "upload" or "download"
	RateMbps  float64 `json:"rate_mbps"`
	// MTR options
	MaxHops      int  `json:"max_hops"`
	ProbesPerHop int  `json:"probes_per_hop"`
	Continuous   bool `json:"continuous"`
	// Profile
	ProfileName string `json:"profile_name"`
}

// ProfilePayload for run_profile action.
type ProfilePayload struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// AnalyzePayload for analyze_session action.
type AnalyzePayload struct {
	SessionID string `json:"session_id"`
}

// KnowledgePayload for ingest_knowledge action.
type KnowledgePayload struct {
	Text string `json:"text"`
}

// SettingsPayload for get/save settings.
type SettingsPayload struct {
	MaxUDPRateMbps  float64 `json:"max_udp_rate_mbps"`
	MaxStreams      int     `json:"max_streams"`
	DefaultTimeout  float64 `json:"default_timeout_ms"`
	DefaultAttempts int     `json:"default_attempts"`
	StoragePath     string  `json:"storage_path"`
	PrivacyMode     bool    `json:"privacy_mode"`
	AIEnabled       bool    `json:"ai_enabled"`
}

// Response sent to the WebSocket client.
type Response struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Global settings (in-memory for now)
var (
	globalSettings = SettingsPayload{
		MaxUDPRateMbps:  100,
		MaxStreams:      8,
		DefaultTimeout:  2000,
		DefaultAttempts: 10,
		StoragePath:     ".",
		PrivacyMode:     false,
		AIEnabled:       true,
	}
	settingsMu sync.RWMutex
)

// Per-client cancel map
var (
	clientCancelMu sync.Mutex
	clientCancels  = map[*Client]context.CancelFunc{}
)

func cancelClientOp(c *Client) {
	clientCancelMu.Lock()
	defer clientCancelMu.Unlock()
	if cancel, ok := clientCancels[c]; ok {
		cancel()
		delete(clientCancels, c)
	}
}

func registerClientCtx(c *Client) (context.Context, context.CancelFunc) {
	cancelClientOp(c) // Cancel any previous running op
	ctx, cancel := context.WithCancel(context.Background())
	clientCancelMu.Lock()
	clientCancels[c] = cancel
	clientCancelMu.Unlock()
	return ctx, cancel
}

func handleClientMessage(c *Client, msgBytes []byte) {
	var msg Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Printf("Invalid message format: %v", err)
		return
	}

	switch msg.Action {
	case ActionStop:
		cancelClientOp(c)
		c.sendJSON("status", "stopped")

	case ActionRunQuick, ActionRunQuickFull:
		var target TargetPayload
		if err := json.Unmarshal(msg.Payload, &target); err != nil {
			c.sendError("Invalid payload for run_quick")
			return
		}
		ctx, cancel := registerClientCtx(c)
		defer cancel()
		go runQuickFull(ctx, c, target)

	case ActionRunMTR, ActionRunMTRContinuous:
		var target TargetPayload
		if err := json.Unmarshal(msg.Payload, &target); err != nil {
			c.sendError("Invalid payload for run_mtr")
			return
		}
		ctx, cancel := registerClientCtx(c)
		defer cancel()
		go runMTR(ctx, c, target)

	case ActionRunReach, ActionRunReachExt:
		var target TargetPayload
		if err := json.Unmarshal(msg.Payload, &target); err != nil {
			c.sendError("Invalid payload for run_reach")
			return
		}
		ctx, cancel := registerClientCtx(c)
		defer cancel()
		go runReachability(ctx, c, target)

	case ActionRunPerf, ActionRunPerfExt:
		var target TargetPayload
		if err := json.Unmarshal(msg.Payload, &target); err != nil {
			c.sendError("Invalid payload for run_perf")
			return
		}
		ctx, cancel := registerClientCtx(c)
		defer cancel()
		go runPerf(ctx, c, target)

	case ActionConsultExpert:
		var kp KnowledgePayload // Re-use text field
		if err := json.Unmarshal(msg.Payload, &kp); err != nil {
			c.sendError("Invalid consultation payload")
			return
		}
		eng := diagnosis.NewEngine()
		if eng.AIExpert != nil {
			result, err := eng.AIExpert.Analyze(context.Background(), kp.Text)
			if err != nil {
				c.sendError(fmt.Sprintf("Consultation failed: %v", err))
			} else {
				c.sendJSON("ai_consultation", result)
			}
		} else {
			c.sendError("AI Expert not initialized")
		}

	case ActionAIGuide:
		var kp KnowledgePayload
		if err := json.Unmarshal(msg.Payload, &kp); err != nil {
			c.sendError("Invalid guide payload")
			return
		}
		go c.handleAIGuide(kp.Text)

	case ActionRunProfile:
		var target TargetPayload
		if err := json.Unmarshal(msg.Payload, &target); err != nil {
			c.sendError("Invalid payload for run_profile")
			return
		}
		ctx, cancel := registerClientCtx(c)
		defer cancel()
		go runProfile(ctx, c, target)

	case ActionAnalyzeSession:
		var ap AnalyzePayload
		if err := json.Unmarshal(msg.Payload, &ap); err != nil {
			c.sendError("Invalid payload for analyze_session")
			return
		}
		go analyzeSession(c, ap)

	case ActionListSessions:
		files, _ := engine.ListSessions(".")
		c.sendJSON("session_list", files)

	case ActionGetSettings:
		settingsMu.RLock()
		s := globalSettings
		settingsMu.RUnlock()
		c.sendJSON("settings", s)

	case ActionSaveSettings:
		var s SettingsPayload
		if err := json.Unmarshal(msg.Payload, &s); err != nil {
			c.sendError("Invalid settings payload")
			return
		}
		settingsMu.Lock()
		globalSettings = s
		settingsMu.Unlock()
		c.sendJSON("settings_saved", "ok")

	case ActionIngestKnowledge:
		var kp KnowledgePayload
		if err := json.Unmarshal(msg.Payload, &kp); err != nil {
			c.sendError("Invalid knowledge payload")
			return
		}
		eng := diagnosis.NewEngine()
		if eng.AIExpert != nil {
			err := eng.AIExpert.IngestKnowledge(context.Background(), kp.Text)
			if err != nil {
				c.sendError(fmt.Sprintf("Failed to ingest knowledge: %v", err))
			} else {
				c.sendJSON("knowledge_ingested", "Expert knowledge updated locally.")
			}
		} else {
			c.sendError("AI Expert not initialized")
		}

	default:
		c.sendError(fmt.Sprintf("Unknown action: %s", msg.Action))
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (c *Client) sendJSON(typeStr string, data interface{}) {
	resp := Response{Type: typeStr, Payload: data}
	b, _ := json.Marshal(resp)
	select {
	case c.send <- b:
	default:
		log.Printf("Client send buffer full, dropping message of type: %s", typeStr)
	}
}

func (c *Client) sendError(errStr string) {
	c.sendJSON("error", errStr)
}

// ─── Feature Runners ──────────────────────────────────────────────────────────

func runQuickFull(ctx context.Context, c *Client, t TargetPayload) {
	c.sendJSON("status", "running")
	c.sendJSON("log", fmt.Sprintf("▶ Quick Check: %s port %d...", t.Host, t.Port))

	diagData := diagnosis.InputData{}

	timeout := 5 * time.Second
	if t.Timeout > 0 {
		timeout = time.Duration(t.Timeout) * time.Millisecond
	}

	// ── helper: run a blocking probe in a goroutine, return result or ctx cancel ──
	type dnsResult struct{ r probes.DNSResult }
	type tcpResult struct{ r probes.TCPResult }
	type tlsResult struct{ r probes.TLSResult }

	// 1. DNS ─────────────────────────────────────────────────────────────────
	c.sendJSON("log", "  → DNS resolution...")
	dnsCh := make(chan probes.DNSResult, 1)
	go func() { dnsCh <- probes.ResolveTimeout(t.Host, timeout) }()

	select {
	case <-ctx.Done():
		c.sendJSON("status", "done")
		return
	case res := <-dnsCh:
		diagData.DNSDuration = float64(res.Duration.Milliseconds())
		if res.Error != nil {
			diagData.DNSSuccess = false
			c.sendJSON("quick_dns", map[string]interface{}{
				"status": "fail",
				"label":  "DNS Resolution",
				"detail": fmt.Sprintf("Failed: %v", res.Error),
			})
		} else {
			diagData.DNSSuccess = true
			c.sendJSON("quick_dns", map[string]interface{}{
				"status": "pass",
				"label":  "DNS Resolution",
				"detail": fmt.Sprintf("%.0fms → %v", diagData.DNSDuration, res.ResolvedIPs),
			})
		}
	}

	// 2. TCP ─────────────────────────────────────────────────────────────────
	c.sendJSON("log", fmt.Sprintf("  → TCP connect %s:%d...", t.Host, t.Port))
	tcpCh := make(chan probes.TCPResult, 1)
	go func() { tcpCh <- probes.ReachTCP(t.Host, t.Port, timeout) }()

	select {
	case <-ctx.Done():
		c.sendJSON("status", "done")
		return
	case res := <-tcpCh:
		rttMs := float64(res.RTT.Milliseconds())
		diagData.TCPSuccess = res.Success
		diagData.ReachabilitySuccess = res.Success
		diagData.ReachabilityRTT = rttMs
		if res.Success {
			status := "pass"
			if rttMs > 200 {
				status = "warn"
			}
			c.sendJSON("quick_tcp", map[string]interface{}{
				"status": status,
				"label":  "TCP Connect",
				"detail": fmt.Sprintf("%.0fms RTT", rttMs),
			})
		} else {
			diagData.ReachabilityError = fmt.Sprintf("%v", res.Error)
			c.sendJSON("quick_tcp", map[string]interface{}{
				"status": "fail",
				"label":  "TCP Connect",
				"detail": fmt.Sprintf("Failed: %v", res.Error),
			})
		}
	}

	// 3. TLS ─────────────────────────────────────────────────────────────────
	// Skip only if TCP failed entirely; try TLS even on unknown ports
	if !diagData.TCPSuccess {
		c.sendJSON("quick_tls", map[string]interface{}{
			"status": "skip",
			"label":  "TLS Handshake",
			"detail": "Skipped (TCP failed)",
		})
	} else {
		c.sendJSON("log", "  → TLS handshake...")
		tlsCh := make(chan probes.TLSResult, 1)
		go func() { tlsCh <- probes.CheckTLS(t.Host, t.Port, timeout) }()

		select {
		case <-ctx.Done():
			c.sendJSON("status", "done")
			return
		case res := <-tlsCh:
			diagData.TLSSuccess = res.Success
			diagData.TLSHandshakeTime = float64(res.HandshakeTime.Milliseconds())
			if res.Success {
				status := "pass"
				if diagData.TLSHandshakeTime > 500 {
					status = "warn"
				}
				c.sendJSON("quick_tls", map[string]interface{}{
					"status": status,
					"label":  "TLS Handshake",
					"detail": fmt.Sprintf("%.0fms (TLS %x)", diagData.TLSHandshakeTime, res.Version),
				})
			} else {
				c.sendJSON("quick_tls", map[string]interface{}{
					"status": "skip",
					"label":  "TLS Handshake",
					"detail": "No TLS (plain TCP or not supported)",
				})
			}
		}
	}

	// 4. AI Analysis ──────────────────────────────────────────────────────────
	settingsMu.RLock()
	aiEnabled := globalSettings.AIEnabled
	privacyMode := globalSettings.PrivacyMode
	settingsMu.RUnlock()

	if aiEnabled {
		eng := diagnosis.NewEngine()
		eng.PrivacyMode = privacyMode
		result := eng.Analyze(diagData)
		c.sendJSON("ai_findings", result)
		c.sendJSON("log", fmt.Sprintf("🤖 AI: %d finding(s) — %s", len(result.Findings), result.Summary[:min(len(result.Summary), 80)]))
	}

	c.sendJSON("log", "✓ Quick Check complete.")
	c.sendJSON("status", "done")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runMTR(ctx context.Context, c *Client, t TargetPayload) {
	c.sendJSON("status", "running")

	maxHops := t.MaxHops
	if maxHops <= 0 {
		maxHops = 30
	}
	timeout := 1 * time.Second
	if t.Timeout > 0 {
		timeout = time.Duration(t.Timeout) * time.Millisecond
	}

	if t.Continuous {
		// Continuous MTR-like mode — loop until context cancelled
		c.sendJSON("log", fmt.Sprintf("▶ Continuous MTR to %s:%d (max %d hops, press Stop to end)", t.Host, t.Port, maxHops))
		for {
			select {
			case <-ctx.Done():
				c.sendJSON("status", "done")
				return
			default:
			}
			probes.TraceTCP(t.Host, t.Port, maxHops, timeout, func(h probes.TraceHop) {
				select {
				case <-ctx.Done():
					return
				default:
					c.sendJSON("mtr_hop", h)
				}
			})
			// Sleep before next pass
			select {
			case <-ctx.Done():
				c.sendJSON("status", "done")
				return
			case <-time.After(2 * time.Second):
			}
		}
	} else {
		c.sendJSON("log", fmt.Sprintf("▶ Trace to %s:%d (max %d hops)...", t.Host, t.Port, maxHops))
		probes.TraceTCP(t.Host, t.Port, maxHops, timeout, func(h probes.TraceHop) {
			select {
			case <-ctx.Done():
				return
			default:
				c.sendJSON("mtr_hop", h)
			}
		})
		c.sendJSON("status", "done")
	}
}

func runReachability(ctx context.Context, c *Client, t TargetPayload) {
	c.sendJSON("status", "running")

	attempts := t.Attempts
	if attempts <= 0 {
		attempts = 60 // default: 60 attempts (1/s = 60s)
	}
	timeout := time.Duration(t.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 1 * time.Second
	}
	interval := time.Duration(t.Interval) * time.Millisecond
	if interval <= 0 {
		interval = 1 * time.Second
	}

	c.sendJSON("log", fmt.Sprintf("▶ Reaching %s:%d (%d probes every %v)...", t.Host, t.Port, attempts, interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	count := 0
	for {
		select {
		case <-ctx.Done():
			c.sendJSON("status", "done")
			return
		case <-ticker.C:
			if count >= attempts {
				c.sendJSON("status", "done")
				return
			}
			count++
			res := probes.ReachTCP(t.Host, t.Port, timeout)
			payload := map[string]interface{}{"rtt": 0.0, "err": "", "seq": count}
			if res.Success {
				payload["rtt"] = float64(res.RTT.Milliseconds())
				payload["status"] = "success"
			} else {
				payload["err"] = fmt.Sprintf("%v", res.Error)
				payload["status"] = "fail"
			}
			c.sendJSON("reach_data", payload)
		}
	}
}

func runPerf(ctx context.Context, c *Client, t TargetPayload) {
	c.sendJSON("status", "running")

	streams := t.Streams
	if streams <= 0 {
		streams = 1
	}
	duration := time.Duration(t.Duration * float64(time.Second))
	if duration <= 0 {
		duration = 10 * time.Second
	}

	c.sendJSON("log", fmt.Sprintf("▶ TCP Perf Client → %s:%d (%d streams, %v)...", t.Host, t.Port, streams, duration))

	intervalIdx := 0
	res := probes.RunTCPClient(t.Host, t.Port, streams, duration, func(bps float64) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		intervalIdx++
		payload := map[string]interface{}{
			"mbps":     bps / 1_000_000.0,
			"interval": intervalIdx,
		}
		c.sendJSON("perf_data", payload)
	})

	select {
	case <-ctx.Done():
		c.sendJSON("status", "done")
		return
	default:
	}

	avgMbps := res.ThroughputBps / 1_000_000.0
	c.sendJSON("perf_final", map[string]interface{}{
		"avg_mbps": avgMbps,
		"bytes":    res.TotalBytes,
	})
	c.sendJSON("log", fmt.Sprintf("✓ Done. Avg: %.2f Mbps", avgMbps))

	// Run AI on perf data if enabled
	settingsMu.RLock()
	aiEnabled := globalSettings.AIEnabled
	privacyMode := globalSettings.PrivacyMode
	settingsMu.RUnlock()

	if aiEnabled {
		diagData := diagnosis.InputData{
			ReachabilitySuccess: true,
			TCPSuccess:          true,
			ThroughputMbps:      avgMbps,
		}
		eng := diagnosis.NewEngine()
		eng.PrivacyMode = privacyMode
		result := eng.Analyze(diagData)
		if len(result.Findings) > 0 {
			c.sendJSON("ai_findings", result)
		}
	}

	c.sendJSON("status", "done")
}

func runProfile(ctx context.Context, c *Client, t TargetPayload) {
	c.sendJSON("status", "running")
	name := t.ProfileName
	if name == "" {
		name = "wan"
	}
	c.sendJSON("log", fmt.Sprintf("▶ Running profile '%s' on %s:%d...", name, t.Host, t.Port))

	target := fmt.Sprintf("%s:%d", t.Host, t.Port)
	session := engine.RunProfile(name, target, func(step string) {
		select {
		case <-ctx.Done():
			return
		default:
			c.sendJSON("log", step)
		}
	})

	c.sendJSON("log", fmt.Sprintf("✓ Profile '%s' completed: %d test(s)", name, len(session.Results)))
	c.sendJSON("status", "done")
}

func analyzeSession(c *Client, ap AnalyzePayload) {
	c.sendJSON("log", fmt.Sprintf("🤖 Analyzing session: %s...", ap.SessionID))

	settingsMu.RLock()
	aiEnabled := globalSettings.AIEnabled
	privacyMode := globalSettings.PrivacyMode
	settingsMu.RUnlock()

	if !aiEnabled {
		c.sendJSON("log", "AI is disabled in settings.")
		return
	}

	// For demo: run analysis on empty data to show engine works
	eng := diagnosis.NewEngine()
	eng.PrivacyMode = privacyMode
	result := eng.Analyze(diagnosis.InputData{})
	c.sendJSON("ai_findings", result)
}

func (c *Client) handleAIGuide(text string) {
	eng := diagnosis.NewEngine()
	if eng.AIExpert == nil {
		c.sendError("AI Expert not initialized")
		return
	}

	ctx := context.Background()
	c.sendJSON("log", "🤖 AI Expert is initiating a guided investigation...")

	// 1. Initial analysis and command request
	narrative, actions, err := eng.AIExpert.AnalyzeGuided(ctx, text)
	if err != nil {
		c.sendError(fmt.Sprintf("AI Guide failed: %v", err))
		return
	}

	c.sendJSON("ai_consultation", narrative)

	if len(actions) == 0 {
		return
	}

	// 2. Execute requested actions
	var resultsSummary []string
	for _, action := range actions {
		c.sendJSON("log", fmt.Sprintf("🚀 AI requested execution: %s %s", action.Command, action.Target))

		var resultStr string
		switch action.Command {
		case "dns":
			res := probes.Resolve(action.Target)
			resultStr = fmt.Sprintf("DNS %s: IPs=%v, Err=%v", action.Target, res.ResolvedIPs, res.Error)
		case "reach":
			// Assume port 443 for simple reach
			res := probes.ReachTCP(action.Target, 443, 2*time.Second)
			resultStr = fmt.Sprintf("Reach %s:443: Success=%v, RTT=%v", action.Target, res.Success, res.RTT)
		case "trace":
			// Simplified trace for guided mode
			hops, _ := probes.TraceTCP(action.Target, 443, 10, 1*time.Second, nil)
			resultStr = fmt.Sprintf("Trace %s: %d hops reached", action.Target, len(hops))
		default:
			resultStr = fmt.Sprintf("Command %s not implemented for guided mode yet", action.Command)
		}

		c.sendJSON("log", fmt.Sprintf("📊 Result: %s", resultStr))
		resultsSummary = append(resultsSummary, resultStr)
	}

	// 3. Final interpretation
	followUp := fmt.Sprintf("The results of my requested actions are:\n%s\n\nBased on this, what is your final diagnosis?", strings.Join(resultsSummary, "\n"))
	finalNarrative, _, _ := eng.AIExpert.AnalyzeGuided(ctx, followUp)
	c.sendJSON("ai_consultation", finalNarrative)
}
