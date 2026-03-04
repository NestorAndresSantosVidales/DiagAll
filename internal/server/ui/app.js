/* ─── app.js — DiagAll UI Logic ──────────────────────────────────────────── */
'use strict';

// ─── WebSocket ────────────────────────────────────────────────────────────────
let ws;
const connStatus = document.getElementById('connStatus');

function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/ws`);

    ws.onopen = () => {
        connStatus.className = 'conn-badge connected';
        connStatus.querySelector('.conn-text').textContent = 'Connected';
        loadSettings();
    };

    ws.onclose = () => {
        connStatus.className = 'conn-badge';
        connStatus.querySelector('.conn-text').textContent = 'Offline';
        setTimeout(connect, 2000);
    };

    ws.onmessage = (ev) => {
        try {
            handleMsg(JSON.parse(ev.data));
        } catch (e) {
            console.warn('Bad WS message:', e);
        }
    };
}

function send(action, payload) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        logConsole('Not connected.', 'err'); return;
    }
    ws.send(JSON.stringify({ action, payload }));
}

// ─── State ────────────────────────────────────────────────────────────────────
const state = {
    currentTab: 'quick',
    running: false,
    mtrHops: {},  // TTL → rolling stats
    reachStats: { sent: 0, ok: 0, fail: 0, rtts: [] },
    perfStats: { vals: [], cur: 0 },
};

// ─── Charts (lazy initialised) ────────────────────────────────────────────────
let chartReach = null;
let chartPerf = null;

function getChartReach() {
    if (!chartReach) chartReach = new TimeSeriesChart('chart-reach-rtt', '#2f81f7');
    return chartReach;
}
function getChartPerf() {
    if (!chartPerf) chartPerf = new PerfChart('chart-perf-mbps');
    return chartPerf;
}

// ─── Tab navigation ───────────────────────────────────────────────────────────
function showTab(id, btn) {
    document.querySelectorAll('.tab-pane').forEach(el => el.classList.remove('active'));
    document.getElementById('tab-' + id)?.classList.add('active');
    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    if (btn) btn.classList.add('active');
    state.currentTab = id;
}

// Jump to tab programmatically from nav button click event
document.querySelectorAll('.nav-item').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
    });
});

// ─── Target bar helpers ───────────────────────────────────────────────────────
function getTarget() {
    return {
        host: document.getElementById('targetHost').value.trim(),
        port: parseInt(document.getElementById('targetPort').value) || 443,
        protocol: document.getElementById('targetProto').value,
        sni: document.getElementById('targetSNI').value.trim(),
    };
}

function validateTarget() {
    const host = document.getElementById('targetHost');
    const port = document.getElementById('targetPort');
    const hostHint = document.getElementById('hostHint');
    const portHint = document.getElementById('portHint');
    let ok = true;

    const hostVal = host.value.trim();
    if (!hostVal) {
        host.classList.add('invalid'); hostHint.textContent = 'Required'; ok = false;
    } else {
        host.classList.remove('invalid'); hostHint.textContent = '';
    }

    const portVal = parseInt(port.value);
    if (isNaN(portVal) || portVal < 1 || portVal > 65535) {
        port.classList.add('invalid'); portHint.textContent = '1–65535'; ok = false;
    } else {
        port.classList.remove('invalid'); portHint.textContent = '';
    }

    return ok;
}

// ─── Start/Stop global ───────────────────────────────────────────────────────
function startCurrentTab() {
    switch (state.currentTab) {
        case 'quick': runQuickCheck(); break;
        case 'reach': runReach(); break;
        case 'path': runMTR(); break;
        case 'perf': runPerf(); break;
        case 'profiles': break;
        default: break;
    }
}

function stopAll() {
    send('stop', {});
    setRunning(false);
}

function setRunning(running) {
    state.running = running;
    const btnStart = document.getElementById('btnStart');
    const btnStop = document.getElementById('btnStop');
    if (running) {
        btnStart.style.display = 'none';
        btnStop.style.display = '';
    } else {
        btnStart.style.display = '';
        btnStop.style.display = 'none';
    }
}

// ─── Message handler ──────────────────────────────────────────────────────────
function handleMsg(msg) {
    switch (msg.type) {
        case 'status':
            if (msg.payload === 'running') setRunning(true);
            if (msg.payload === 'done' || msg.payload === 'stopped') setRunning(false);
            break;

        case 'log':
            logConsole(msg.payload);
            break;

        case 'error':
            logConsole('ERROR: ' + msg.payload, 'err');
            setRunning(false);
            break;

        // Quick Check cards
        case 'quick_dns': updateQuickCard('card-dns', msg.payload); break;
        case 'quick_tcp': updateQuickCard('card-tcp', msg.payload); break;
        case 'quick_tls': updateQuickCard('card-tls', msg.payload); break;

        // AI findings
        case 'ai_findings':
            renderFindingsPanel(msg.payload, 'findingsPanel', 'findingsList', 'findingsSummary', 'findingsBadge', 'findingsSteps', 'nextStepsList');
            break;

        // MTR hops
        case 'mtr_hop':
        case 'hop':
            updateMTRRow(msg.payload);
            break;

        // Reachability
        case 'reach_data':
            updateReach(msg.payload);
            break;

        // Performance intervals
        case 'perf_data':
            updatePerf(msg.payload);
            break;

        case 'perf_final':
            logConsole(`✓ Final avg: ${msg.payload.avg_mbps?.toFixed(2)} Mbps`);
            break;

        case 'session_list':
            renderSessionList(msg.payload);
            break;

        case 'settings':
            applySettings(msg.payload);
            break;

        case 'settings_saved':
            logConsole('Settings saved.', 'ok');
            break;

        case 'knowledge_ingested':
            logToConsole('aiChatConsole', 'System: ' + msg.payload, 'system');
            document.getElementById('aiKnowledgeInput').value = '';
            alert('Expert knowledge updated locally.');
            break;

        case 'ai_consultation':
            addChatMsg('ai', msg.payload);
            break;
    }
}

// ─── Quick Check ─────────────────────────────────────────────────────────────
function runQuickCheck() {
    if (!validateTarget()) return;
    const t = getTarget();
    resetQuickCards();
    document.getElementById('findingsPanel').style.display = 'none';
    clearConsole('console-quick');
    setRunning(true);

    send('run_quick_full', {
        host: t.host,
        port: t.port,
        protocol: t.protocol,
        sni: t.sni,
        timeout_ms: 2000,
    });
}

function resetQuickCards() {
    ['card-dns', 'card-tcp', 'card-tls'].forEach(id => {
        const card = document.getElementById(id);
        card.className = 'status-card';
        card.querySelector('.card-value').className = 'card-value pending';
        card.querySelector('.card-value').textContent = '—';
        card.querySelector('.card-detail').textContent = '';
        card.querySelector('.card-badge').className = 'card-badge running';
        card.querySelector('.card-badge').textContent = 'Running…';
    });
}

function updateQuickCard(id, payload) {
    const card = document.getElementById(id);
    if (!card) return;
    const status = payload.status; // pass | fail | warn | skip
    const value = document.createElement('span');
    const valEl = card.querySelector('.card-value');
    const detEl = card.querySelector('.card-detail');
    const badgeEl = card.querySelector('.card-badge');

    card.className = 'status-card ' + status;
    valEl.className = 'card-value ' + status;
    badgeEl.className = 'card-badge ' + status;

    const icons = { pass: '✓', fail: '✗', warn: '⚠', skip: '—' };
    valEl.textContent = icons[status] || status;
    detEl.textContent = payload.detail || '';
    badgeEl.textContent = { pass: 'OK', fail: 'FAIL', warn: 'WARN', skip: 'SKIP' }[status] || status.toUpperCase();
}

// ─── AI Findings renderer ────────────────────────────────────────────────────
function renderFindingsPanel(result, panelId, listId, summaryId, badgeId, stepsId, stepsListId) {
    const panel = document.getElementById(panelId);
    const list = document.getElementById(listId);
    const summary = document.getElementById(summaryId);
    const badge = document.getElementById(badgeId);
    const steps = document.getElementById(stepsId);
    const stepsList = document.getElementById(stepsListId);

    if (!panel) return;

    const findings = result.findings || [];
    panel.style.display = '';

    if (badge) badge.textContent = `${findings.length} finding${findings.length !== 1 ? 's' : ''}`;

    // Executive summary
    if (summary && result.summary) {
        summary.style.display = '';
        summary.innerHTML = formatMarkdown(result.summary);
    }

    // Findings list
    if (list) {
        list.innerHTML = '';
        findings.forEach((f, i) => {
            const el = document.createElement('div');
            el.className = `finding-item severity-${f.severity}`;
            el.innerHTML = `
        <div class="finding-header" onclick="toggleFinding(this)">
          <span class="finding-title">${escHtml(f.title)}</span>
          <div style="display:flex;gap:8px;align-items:center">
            <span class="sev-badge ${f.severity}">${f.severity}</span>
            <span style="color:var(--text-muted);font-size:.75rem">▼</span>
          </div>
        </div>
        <div class="finding-body open">
          <p class="finding-desc">${escHtml(f.description)}</p>
          <div class="confidence-row">
            <span class="confidence-label">Confidence</span>
            <div class="confidence-track">
              <div class="confidence-fill" style="width:${Math.round(f.confidence * 100)}%"></div>
            </div>
            <span class="confidence-label">${Math.round(f.confidence * 100)}%</span>
          </div>
          ${f.evidence?.length ? `<div class="finding-section-title">Evidence</div><ul>${f.evidence.map(e => `<li>${escHtml(e)}</li>`).join('')}</ul>` : ''}
          ${f.probable_causes?.length ? `<div class="finding-section-title">Probable Causes</div><ul>${f.probable_causes.map(c => `<li>${escHtml(c)}</li>`).join('')}</ul>` : ''}
          ${f.recommended_actions?.length ? `<div class="finding-section-title">Recommended Actions</div><ul>${f.recommended_actions.map(a => `<li>${escHtml(a)}</li>`).join('')}</ul>` : ''}
        </div>
      `;
            list.appendChild(el);
        });
    }

    // Next steps
    if (steps && stepsListId && result.next_steps?.length) {
        steps.style.display = '';
        const ol = document.getElementById(stepsListId);
        if (ol) {
            ol.innerHTML = result.next_steps.map(s => `<li>${escHtml(s)}</li>`).join('');
        }
    }
}

function toggleFinding(header) {
    const body = header.nextElementSibling;
    if (!body) return;
    body.classList.toggle('open');
    const arrow = header.querySelector('span:last-child');
    if (arrow) arrow.textContent = body.classList.contains('open') ? '▼' : '▶';
}

// ─── Reachability ─────────────────────────────────────────────────────────────
function runReach() {
    if (!validateTarget()) return;
    const t = getTarget();
    clearConsole('console-reach'); // no console-reach but okay
    state.reachStats = { sent: 0, ok: 0, fail: 0, rtts: [] };
    updateReachStats();
    document.querySelector('#reach-table-body').innerHTML = '';
    getChartReach().reset();
    setRunning(true);

    send('run_reach_extended', {
        host: t.host,
        port: t.port,
        protocol: t.protocol,
        attempts: parseInt(document.getElementById('reachAttempts').value) || 60,
        timeout_ms: parseFloat(document.getElementById('reachTimeout').value) || 2000,
        interval_ms: parseFloat(document.getElementById('reachInterval').value) || 1000,
    });
}

function updateReach(payload) {
    const s = state.reachStats;
    s.sent++;
    if (payload.status === 'success') {
        s.ok++;
        s.rtts.push(payload.rtt);
        getChartReach().addValue(payload.rtt);
    } else {
        s.fail++;
        getChartReach().addValue(0);
    }

    // Add table row
    const tbody = document.getElementById('reach-table-body');
    if (tbody) {
        const tr = document.createElement('tr');
        const status = payload.status === 'success'
            ? `<span class="hop-stable">✓ OK</span>`
            : `<span class="hop-spike">✗ FAIL</span>`;
        tr.innerHTML = `<td>${s.sent}</td><td>${status}</td><td>${payload.rtt ? payload.rtt.toFixed(1) : '—'}</td><td>${new Date().toLocaleTimeString()}</td>`;
        tbody.insertBefore(tr, tbody.firstChild);
        if (tbody.rows.length > 100) tbody.deleteRow(-1); // Keep last 100
    }

    updateReachStats();
}

function updateReachStats() {
    const s = state.reachStats;
    const rtts = s.rtts;
    const loss = s.sent > 0 ? ((s.fail / s.sent) * 100).toFixed(1) : '0.0';
    const avg = rtts.length ? (rtts.reduce((a, b) => a + b, 0) / rtts.length).toFixed(1) : '—';
    const min = rtts.length ? Math.min(...rtts).toFixed(1) : '—';
    const max = rtts.length ? Math.max(...rtts).toFixed(1) : '—';

    setText('reach-sent', s.sent);
    setText('reach-ok', s.ok);
    setText('reach-fail', s.fail);
    setText('reach-loss', loss + '%');
    setText('reach-avg', avg ? avg + ' ms' : '—');
    setText('reach-min', min !== '—' ? min + ' ms' : '—');
    setText('reach-max', max !== '—' ? max + ' ms' : '—');
}

// ─── MTR / Path ───────────────────────────────────────────────────────────────
function runMTR() {
    if (!validateTarget()) return;
    const t = getTarget();
    state.mtrHops = {};
    document.getElementById('mtr-body').innerHTML = '';
    document.getElementById('mtrFindingsPanel').style.display = 'none';
    setRunning(true);

    const continuous = document.getElementById('mtrMode').value === 'continuous';

    send(continuous ? 'run_mtr_continuous' : 'run_mtr', {
        host: t.host,
        port: t.port,
        max_hops: parseInt(document.getElementById('mtrMaxHops').value) || 30,
        timeout_ms: parseFloat(document.getElementById('mtrTimeout').value) || 1000,
        continuous: continuous,
    });
}

function updateMTRRow(hop) {
    const ttl = hop.TTL || hop.ttl;
    if (!ttl) return;

    // Update rolling stats in state
    if (!state.mtrHops[ttl]) {
        state.mtrHops[ttl] = {
            host: hop.Host || hop.host || '*',
            rtts: [], sent: 0, lost: 0, last: 0,
        };
    }
    const hd = state.mtrHops[ttl];
    hd.sent++;

    const rttNs = hop.RTT || hop.rtt || 0;
    const rttMs = rttNs / 1_000_000;
    const timeout = hop.Timeout || hop.timeout;

    if (timeout) {
        hd.lost++;
    } else {
        hd.last = rttMs;
        hd.rtts.push(rttMs);
        if (hop.Host || hop.host) hd.host = hop.Host || hop.host;
    }

    const rtts = hd.rtts;
    const avg = rtts.length ? (rtts.reduce((a, b) => a + b, 0) / rtts.length) : 0;
    const best = rtts.length ? Math.min(...rtts) : 0;
    const worst = rtts.length ? Math.max(...rtts) : 0;
    const p95 = rtts.length ? percentile(rtts, 95) : 0;
    const jitter = rtts.length > 1 ? calcJitter(rtts) : 0;
    const loss = hd.sent > 0 ? ((hd.lost / hd.sent) * 100) : 0;

    // Determine status
    let statusClass = 'hop-unknown';
    let statusText = '?';
    if (hd.sent > 0 && rtts.length > 0) {
        if (loss > 20 || (p95 > avg * 3 && rtts.length > 3)) {
            statusClass = 'hop-spike'; statusText = 'Spike';
        } else if (loss > 5) {
            statusClass = 'hop-warning'; statusText = 'Loss';
        } else {
            statusClass = 'hop-stable'; statusText = 'OK';
        }
    }

    const fmt = v => v > 0 ? v.toFixed(1) : '—';

    let row = document.getElementById(`mtr-row-${ttl}`);
    if (!row) {
        row = document.createElement('tr');
        row.id = `mtr-row-${ttl}`;
        document.getElementById('mtr-body').appendChild(row);
        // Sort rows by TTL
        const rows = Array.from(document.querySelectorAll('[id^="mtr-row-"]'));
        rows.sort((a, b) => parseInt(a.id.split('-')[2]) - parseInt(b.id.split('-')[2]));
        const tbody = document.getElementById('mtr-body');
        rows.forEach(r => tbody.appendChild(r));
    }

    row.innerHTML = `
    <td>${ttl}</td>
    <td style="font-family:inherit;font-size:.79rem;max-width:180px;overflow:hidden;text-overflow:ellipsis">${escHtml(hd.host)}</td>
    <td class="${loss > 5 ? 'hop-spike' : ''}">${loss.toFixed(1)}%</td>
    <td>${hd.sent}</td>
    <td>${fmt(hd.last)} ms</td>
    <td>${fmt(avg)} ms</td>
    <td>${fmt(best)} ms</td>
    <td>${fmt(worst)} ms</td>
    <td>${fmt(p95)} ms</td>
    <td>${fmt(jitter)} ms</td>
    <td><span class="${statusClass}">${statusText}</span></td>
  `;
}

function percentile(arr, p) {
    const sorted = [...arr].sort((a, b) => a - b);
    const idx = Math.ceil((p / 100) * sorted.length) - 1;
    return sorted[Math.max(0, idx)];
}

function calcJitter(arr) {
    if (arr.length < 2) return 0;
    let sum = 0;
    for (let i = 1; i < arr.length; i++) sum += Math.abs(arr[i] - arr[i - 1]);
    return sum / (arr.length - 1);
}

// ─── Performance ──────────────────────────────────────────────────────────────
function runPerf() {
    if (!validateTarget()) return;
    const t = getTarget();
    getChartPerf().reset();
    state.perfStats = { vals: [], cur: 0 };
    clearConsole('console-perf');
    document.getElementById('perfFindingsPanel').style.display = 'none';
    logToConsole('console-perf', `▶ Connecting to ${t.host}:${t.port}...`);
    setRunning(true);

    send('run_perf_extended', {
        host: t.host,
        port: t.port,
        streams: parseInt(document.getElementById('perfStreams').value) || 1,
        duration_s: parseFloat(document.getElementById('perfDuration').value) || 10,
        warmup_s: parseFloat(document.getElementById('perfWarmup').value) || 0,
        direction: document.getElementById('perfDir').value,
    });
}

function updatePerf(payload) {
    const mbps = payload.mbps || 0;
    state.perfStats.cur = mbps;
    state.perfStats.vals.push(mbps);

    getChartPerf().addInterval(mbps, 'total');

    const vals = state.perfStats.vals;
    const avg = vals.reduce((a, b) => a + b, 0) / vals.length;
    const max = Math.max(...vals);
    const min = Math.min(...vals);

    setText('perf-cur', mbps.toFixed(2) + ' Mbps');
    setText('perf-avg', avg.toFixed(2) + ' Mbps');
    setText('perf-max', max.toFixed(2) + ' Mbps');
    setText('perf-min', min.toFixed(2) + ' Mbps');
    logToConsole('console-perf', `[${vals.length}s] ${mbps.toFixed(2)} Mbps`);
}

function setPerfMode(mode, btn) {
    document.querySelectorAll('.mode-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('perfClientPanel').style.display = mode === 'client' ? '' : 'none';
    document.getElementById('perfServerPanel').style.display = mode === 'server' ? '' : 'none';
}

function startServer() {
    const port = parseInt(document.getElementById('serverPort').value) || 5201;
    logToConsole('console-server', `Starting TCP server on port ${port}…`);
    // Server-start via REST endpoint
    fetch(`/api/server?port=${port}&proto=tcp`).catch(() => { });
}

// ─── Profiles ─────────────────────────────────────────────────────────────────
function runProfile(name) {
    if (!validateTarget()) return;
    const t = getTarget();
    clearConsole('console-profiles');
    document.getElementById('profileProgress').style.display = '';
    document.getElementById('profileProgressLabel').textContent = `Running ${name.toUpperCase()} profile…`;
    document.getElementById('profileProgressBar').style.width = '0%';
    setRunning(true);

    // Animate progress bar
    let pct = 0;
    const progTimer = setInterval(() => {
        pct = Math.min(pct + 2, 95);
        document.getElementById('profileProgressBar').style.width = pct + '%';
    }, 300);

    send('run_profile', {
        host: t.host,
        port: t.port,
        profile_name: name,
    });

    // Watch for done status to stop progress
    const origHandler = ws.onmessage;
    ws.onmessage = (ev) => {
        try {
            const msg = JSON.parse(ev.data);
            if (msg.type === 'status' && (msg.payload === 'done' || msg.payload === 'stopped')) {
                clearInterval(progTimer);
                document.getElementById('profileProgressBar').style.width = '100%';
                setTimeout(() => { document.getElementById('profileProgress').style.display = 'none'; }, 1500);
            }
            logToConsole('console-profiles', msg.type === 'log' ? msg.payload : '');
            handleMsg(msg);
        } catch (e) { }
        ws.onmessage = origHandler;
    };
}

function showCustomBuilder() {
    logToConsole('console-profiles', 'Custom profile builder coming soon.');
}

function saveAsProfile() {
    const t = getTarget();
    logToConsole('console-profiles', `Profile saved: ${t.host}:${t.port} (${t.protocol.toUpperCase()})`);
}

// ─── Reports ──────────────────────────────────────────────────────────────────
function refreshReports() {
    send('list_sessions', {});
}

function renderSessionList(files) {
    const wrap = document.getElementById('session-list');
    const empty = document.getElementById('session-list-empty');
    wrap.innerHTML = '';
    if (!files || files.length === 0) {
        empty.style.display = '';
        return;
    }
    empty.style.display = 'none';
    files.forEach(f => {
        const reportFile = f.replace('session_', 'report_').replace('.json', '.html');
        const sessionId = f.replace('session_', '').replace('.json', '');
        const div = document.createElement('div');
        div.className = 'session-item';
        div.innerHTML = `
      <div class="session-meta">
        <div class="session-target">${escHtml(f)}</div>
        <div class="session-id">${escHtml(sessionId)}</div>
      </div>
      <div class="session-actions">
        <a class="btn btn-ghost" href="/${reportFile}" target="_blank" style="text-decoration:none">📄 Open</a>
        <button class="btn btn-ghost" onclick="analyzeSession('${escHtml(sessionId)}')">🤖 Re-analyze</button>
      </div>
    `;
        wrap.appendChild(div);
    });
}

function analyzeSession(id) {
    send('analyze_session', { session_id: id });
}

// ─── Settings ────────────────────────────────────────────────────────────────
function loadSettings() {
    send('get_settings', {});
}

function applySettings(s) {
    setVal('set-maxUDP', s.max_udp_rate_mbps);
    setVal('set-maxStreams', s.max_streams);
    setVal('set-timeout', s.default_timeout_ms);
    setVal('set-attempts', s.default_attempts);
    setVal('set-storage', s.storage_path);
    document.getElementById('set-privacy').checked = !!s.privacy_mode;
    document.getElementById('set-ai').checked = s.ai_enabled !== false;
}

function saveSettings() {
    send('save_settings', {
        max_udp_rate_mbps: parseFloat(getVal('set-maxUDP')) || 100,
        max_streams: parseInt(getVal('set-maxStreams')) || 8,
        default_timeout_ms: parseFloat(getVal('set-timeout')) || 2000,
        default_attempts: parseInt(getVal('set-attempts')) || 10,
        storage_path: getVal('set-storage') || '.',
        privacy_mode: document.getElementById('set-privacy').checked,
        ai_enabled: document.getElementById('set-ai').checked,
    });
}

// ─── Console helpers ──────────────────────────────────────────────────────────
function logConsole(text, cls) {
    // Route to active tab's console
    const tabId = state.currentTab;
    const consoleIds = {
        quick: 'console-quick',
        perf: 'console-perf',
        profiles: 'console-profiles',
        server: 'console-server',
        'ai-expert': 'aiChatConsole',
    };
    const cid = consoleIds[tabId] || 'console-quick';
    logToConsole(cid, text, cls);
}

function logToConsole(id, text, cls) {
    const el = document.getElementById(id);
    if (!el || !text) return;
    const line = document.createElement('div');
    line.className = 'console-line' + (cls ? ' ' + cls : '');
    line.textContent = text;
    el.appendChild(line);
    el.scrollTop = el.scrollHeight;
}

function clearConsole(id) {
    const el = document.getElementById(id);
    if (el) el.innerHTML = '';
}

// ─── Utilities ────────────────────────────────────────────────────────────────
function escHtml(str) {
    if (typeof str !== 'string') return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function formatMarkdown(text) {
    // Very basic markdown: **bold** → <strong>
    return escHtml(text)
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\n/g, '<br>');
}

function setText(id, val) {
    const el = document.getElementById(id);
    if (el) el.textContent = val;
}

function setVal(id, val) {
    const el = document.getElementById(id);
    if (el && val !== undefined) el.value = val;
}

function getVal(id) {
    return document.getElementById(id)?.value ?? '';
}

// ─── AI Expert Tab ───────────────────────────────────────────────────────────
function ingestKnowledge() {
    const text = document.getElementById('aiKnowledgeInput').value.trim();
    if (!text) return;
    send('ingest_knowledge', { text });
}

function consultExpert() {
    const input = document.getElementById('aiChatInput');
    const text = input.value.trim();
    if (!text) return;

    addChatMsg('user', text);
    input.value = '';
    send('consult_expert', { text });
}

function startAIGuide() {
    const input = document.getElementById('aiChatInput');
    const text = input.value.trim();
    if (!text) {
        alert("Please enter a question or problem description first.");
        return;
    }

    addChatMsg('user', text);
    input.value = '';
    logToConsole('aiChatConsole', '🤖 AI Guided Investigation started...', 'system');
    send('ai_guide', { text });
}

function addChatMsg(sender, text) {
    const console = document.getElementById('aiChatConsole');
    const msg = document.createElement('div');
    msg.className = 'chat-msg ' + sender;
    msg.innerText = text;
    console.appendChild(msg);
    console.scrollTop = console.scrollHeight;
}

// ─── Init ─────────────────────────────────────────────────────────────────────
connect();
// Load reports on Reports tab open
document.getElementById('nav-reports')?.addEventListener('click', () => setTimeout(refreshReports, 100));
