package report

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"diagall/internal/diagnosis"
)

// SessionData holds all data for a diagnostic session.
type SessionData struct {
	ID        string
	Timestamp time.Time
	Target    string
	Profile   string
	Results   []TestResult
	Diagnosis diagnosis.AnalysisResult
}

// TestResult represents a single test outcome.
type TestResult struct {
	Name    string
	Status  string // PASS, FAIL, SKIP
	Details string
	Metric  string // e.g., "15ms", "50 Mbps"
}

// GenerateReport creates HTML and JSON reports.
func GenerateReport(session SessionData) error {
	// 1. JSON Export
	jsonData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	jsonFile := fmt.Sprintf("session_%s.json", session.ID)
	err = os.WriteFile(jsonFile, jsonData, 0644)
	if err != nil {
		return err
	}

	// 2. HTML Export
	htmlContent := generateHTML(session)
	htmlFile := fmt.Sprintf("report_%s.html", session.ID)
	err = os.WriteFile(htmlFile, []byte(htmlContent), 0644)
	if err != nil {
		return err
	}

	fmt.Printf("Reports generated: %s, %s\n", jsonFile, htmlFile)
	return nil
}

// GenerateZIP bundles session JSON + HTML report into a .zip archive.
func GenerateZIP(session SessionData) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Add JSON
	jsonData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, err
	}
	jf, err := w.Create(fmt.Sprintf("session_%s.json", session.ID))
	if err != nil {
		return nil, err
	}
	jf.Write(jsonData)

	// Add HTML
	htmlData := generateHTML(session)
	hf, err := w.Create(fmt.Sprintf("report_%s.html", session.ID))
	if err != nil {
		return nil, err
	}
	hf.Write([]byte(htmlData))

	w.Close()
	return buf.Bytes(), nil
}

func severityColor(severity string) string {
	switch severity {
	case "HIGH", "CRITICAL":
		return "#d73a49"
	case "MEDIUM":
		return "#b08800"
	case "LOW":
		return "#0366d6"
	default:
		return "#444c56"
	}
}

func generateHTML(session SessionData) string {
	h := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>DiagAll Report - %s</title>
<style>
  :root{--bg:#0d1117;--card:#161b22;--border:#30363d;--text:#c9d1d9;--accent:#1f6feb;--green:#238636;--red:#da3633;--amber:#b08800}
  body{font-family:'Segoe UI',sans-serif;background:var(--bg);color:var(--text);margin:0;padding:24px}
  h1{color:#fff;border-bottom:2px solid var(--accent);padding-bottom:8px}
  h2{color:#e6edf3;margin-top:32px}
  .meta{display:flex;gap:32px;margin:16px 0;padding:16px;background:var(--card);border:1px solid var(--border);border-radius:8px}
  .meta-item label{font-size:.75rem;color:#8b949e;display:block}
  .meta-item span{font-size:1rem;color:#e6edf3;font-weight:600}
  .finding{border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:12px;border-left-width:4px}
  .finding h3{margin:0 0 8px;font-size:1rem}
  .finding p{margin:4px 0;font-size:.9rem;color:#8b949e}
  .badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:.75rem;font-weight:700;margin-left:8px}
  .confidence-bar{height:6px;border-radius:3px;background:#21262d;margin:8px 0}
  .confidence-fill{height:6px;border-radius:3px;background:var(--accent)}
  ul{margin:4px 0;padding-left:20px;font-size:.85rem;color:#8b949e}
  table{width:100%%;border-collapse:collapse;margin-top:8px}
  th,td{border:1px solid var(--border);padding:8px 12px;text-align:left;font-size:.85rem}
  th{background:var(--card);color:#8b949e}
  .pass{color:var(--green)}.fail{color:var(--red)}.skip{color:#444c56}
  .disclaimer{background:#161b22;border:1px solid var(--border);border-radius:6px;padding:12px;font-size:.8rem;color:#8b949e;margin-top:24px}
</style>
</head>
<body>
<h1>DiagAll Diagnostic Report</h1>
<div class="meta">
  <div class="meta-item"><label>Session ID</label><span>%s</span></div>
  <div class="meta-item"><label>Timestamp</label><span>%s</span></div>
  <div class="meta-item"><label>Target</label><span>%s</span></div>
  <div class="meta-item"><label>Profile</label><span>%s</span></div>
</div>
`, session.ID, session.ID, session.Timestamp.Format("2006-01-02 15:04:05"), session.Target, session.Profile)

	// AI Analysis Section
	if len(session.Diagnosis.Findings) > 0 {
		h += "<h2>🤖 Automated Analysis <span style='font-size:.75rem;color:#8b949e;font-weight:400'>(Offline AI)</span></h2>\n"

		// Summary if present
		if session.Diagnosis.Summary != "" {
			h += fmt.Sprintf(`<div style="background:var(--card);border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px;font-size:.9rem;line-height:1.6">%s</div>`, session.Diagnosis.Summary)
		}

		for _, f := range session.Diagnosis.Findings {
			color := severityColor(f.Severity)
			confPct := int(f.Confidence * 100)
			h += fmt.Sprintf(`<div class="finding" style="border-left-color:%s">`, color)
			h += fmt.Sprintf(`<h3>%s <span class="badge" style="background:%s22;color:%s">%s</span></h3>`, f.Title, color, color, f.Severity)
			h += fmt.Sprintf(`<p>%s</p>`, f.Description)
			h += fmt.Sprintf(`<div style="font-size:.8rem;color:#8b949e;margin:4px 0">Confidence: %d%%</div>`, confPct)
			h += fmt.Sprintf(`<div class="confidence-bar"><div class="confidence-fill" style="width:%d%%"></div></div>`, confPct)
			if len(f.Evidence) > 0 {
				h += "<strong style='font-size:.8rem'>Evidence:</strong><ul>"
				for _, e := range f.Evidence {
					h += fmt.Sprintf("<li>%s</li>", e)
				}
				h += "</ul>"
			}
			if len(f.ProbableCauses) > 0 {
				h += "<strong style='font-size:.8rem'>Probable Causes:</strong><ul>"
				for _, c := range f.ProbableCauses {
					h += fmt.Sprintf("<li>%s</li>", c)
				}
				h += "</ul>"
			}
			if len(f.RecommendedActions) > 0 {
				h += "<strong style='font-size:.8rem'>Recommended Actions:</strong><ul>"
				for _, r := range f.RecommendedActions {
					h += fmt.Sprintf("<li>%s</li>", r)
				}
				h += "</ul>"
			}
			h += "</div>\n"
		}

		if len(session.Diagnosis.NextSteps) > 0 {
			h += "<h2>📋 Next Diagnostic Steps</h2><ul>"
			for _, s := range session.Diagnosis.NextSteps {
				h += fmt.Sprintf("<li>%s</li>", s)
			}
			h += "</ul>"
		}

		h += `<div class="disclaimer">⚠️ <strong>Disclaimer:</strong> This analysis is generated automatically based on observed metrics and known network heuristics. It does not replace manual expert validation. Confidence levels reflect internal heuristic certainty, not absolute truth.</div>`
	}

	// Raw Results Table
	h += `<h2>Raw Test Results</h2>
<table>
<tr><th>Test</th><th>Status</th><th>Metric</th><th>Details</th></tr>
`
	for _, r := range session.Results {
		class := "skip"
		if r.Status == "PASS" {
			class = "pass"
		} else if r.Status == "FAIL" {
			class = "fail"
		}
		h += fmt.Sprintf("<tr><td>%s</td><td class='%s'>%s</td><td>%s</td><td>%s</td></tr>\n",
			r.Name, class, r.Status, r.Metric, r.Details)
	}
	h += `</table>
</body>
</html>`

	return h
}
