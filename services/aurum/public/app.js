// ═══════════════════════════════════════════════════════════
// STRATEGIUM — Astartes Primaris Dashboard
// ═══════════════════════════════════════════════════════════

const App = {
  state: {
    view: 'throne',
    fortresses: [],
    marines: [],
    events: [],
    connected: false,
    // Performance
    performance: null,
    trades: [],
    positions: [],
    // Council
    roadmap: null,
    accounts: [],
    payouts: [],
    budget: null,
    allocations: [],
    withdrawalAdvice: [],
    metrics: null,
    // Goals & Billing
    goals: [],
    expenses: [],
    billing: null,
    unifiedBudgets: [],
  },

  // ─── Init ──────────────────────────────────────────────

  async init() {
    // Check auth first
    try {
      await this.api('/auth-check');
    } catch (e) {
      document.getElementById('loginOverlay').classList.remove('hidden');
      this.initGoogleSignIn();
      return;
    }
    this.startApp();
  },

  initGoogleSignIn() {
    const tryInit = () => {
      if (typeof google === 'undefined' || !google.accounts) {
        setTimeout(tryInit, 100);
        return;
      }
      google.accounts.id.initialize({
        client_id: '1044819155969-56ubqgqpferflutn7koa77jkqhks0752.apps.googleusercontent.com',
        callback: (response) => App.handleGoogleLogin(response),
      });
      google.accounts.id.renderButton(
        document.getElementById('googleSignIn'),
        { theme: 'filled_black', size: 'large', width: 300, text: 'signin_with' }
      );
    };
    tryInit();
  },

  startApp() {
    this.bindNav();
    this.bindKeyboard();
    this.connectSSE();
    this.pollStatus();
    this.loadData();
    this.loadCouncil();
    this.loadHoldings();
    this.loadWheelCycles();
  },

  // ─── API Client ────────────────────────────────────────

  async api(path, opts = {}) {
    try {
      const res = await fetch(`/api/v1${path}`, {
        headers: { 'Content-Type': 'application/json' },
        ...opts,
        body: opts.body ? JSON.stringify(opts.body) : undefined,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || res.statusText);
      }
      return res.json();
    } catch (e) {
      console.error(`API ${path}:`, e);
      throw e;
    }
  },

  // ─── SSE Connection ───────────────────────────────────

  connectSSE() {
    const es = new EventSource('/ws');
    const statusEl = document.getElementById('connStatus');

    es.onopen = () => {
      this.state.connected = true;
      statusEl.className = 'connection-status connected';
      statusEl.querySelector('.status-text').textContent = 'CONNECTED';
    };

    es.addEventListener('connected', () => {
      this.state.connected = true;
      statusEl.className = 'connection-status connected';
      statusEl.querySelector('.status-text').textContent = 'LIVE';
    });

    es.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data);
        this.handleEvent(event);
      } catch (err) {
        console.warn('SSE parse error:', err);
      }
    };

    es.onerror = () => {
      this.state.connected = false;
      statusEl.className = 'connection-status error';
      statusEl.querySelector('.status-text').textContent = 'OFFLINE';
    };
  },

  // ─── Event Handler ────────────────────────────────────

  handleEvent(event) {
    this.state.events.unshift(event);
    if (this.state.events.length > 200) this.state.events.length = 200;

    this.renderEventFeed();
    this.loadData(); // Refresh data on events

    // Update marine status in real-time
    if (event.event === 'wake' || event.event === 'sleep' || event.event === 'failed') {
      this.updateMarineIndicator(event.marine_id, event.event);
    }

    // Combine passed (evaluation cleared, not yet funded)
    if (event.event === 'combine.passed') {
      const name = (event.data && (event.data.account_name || event.data.account_id)) || 'account';
      this.flash(`Combine passed — ${name}`, 'success');
      this.loadCouncil();
    }

    // Rubicon — account funded (real money, crossed from combine/sim to live capital)
    if (event.event === 'account.funded') {
      const name = (event.data && (event.data.account_name || event.data.account_id)) || 'account';
      const firm = event.data && event.data.prop_firm;
      this.flash(`🩸 Rubicon crossed — ${name}${firm ? ' @ ' + firm : ''} FUNDED`, 'success');
      this.loadCouncil();
    }
  },

  updateMarineIndicator(marineId, event) {
    const dot = document.querySelector(`[data-marine-id="${marineId}"] .marine-status-dot`);
    if (!dot) return;
    dot.className = 'marine-status-dot';
    switch (event) {
      case 'wake': dot.classList.add('waking'); break;
      case 'sleep': dot.classList.add('dormant'); break;
      case 'failed': dot.classList.add('failed'); break;
    }
  },

  // ─── Data Loading ─────────────────────────────────────

  async loadData() {
    try {
      const [fortresses, marines, performance, trades, positions] = await Promise.all([
        this.api('/fortresses'),
        this.api('/marines'),
        this.api('/performance').catch(() => null),
        this.api('/trades').catch(() => []),
        this.api('/positions').catch(() => []),
      ]);
      this.state.fortresses = fortresses || [];
      this.state.marines = marines || [];
      this.state.performance = performance;
      this.state.trades = trades || [];
      this.state.positions = positions || [];
      this.renderThrone();
      this.renderTactical();
      this.renderPerformance();
      this.renderStatusCards();
    } catch (e) {
      // Primarch might not be running yet
    }
  },

  async pollStatus() {
    const poll = async () => {
      try {
        const status = await this.api('/status');
        this.renderStatus(status);
      } catch (e) {
        document.getElementById('primarchStatus').textContent = 'OFFLINE';
      }
    };
    poll();
    setInterval(poll, 5000);
  },

  // ─── Render: Status Cards ─────────────────────────────

  renderStatus(status) {
    document.getElementById('primarchStatus').textContent = 'ONLINE';
    document.getElementById('primarchUptime').textContent = status.uptime || '';
    document.getElementById('marineCount').textContent = status.marines?.total || 0;
    document.getElementById('activeMarines').textContent = status.marines?.active || 0;

    // Summary banner
    setText('sumFortresses', status.fortresses || 0);
    setText('sumMarines', status.marines?.total || 0);
    setText('sumActive', status.marines?.active || 0);
  },

  renderStatusCards() {
    const p = this.state.performance;
    if (!p) return;

    // Today's P&L — find today in daily_pnl
    const today = new Date().toISOString().slice(0, 10);
    const todayData = (p.daily_pnl || []).find(d => d.date === today);
    const todayPnl = todayData ? todayData.pnl : 0;
    const todayTrades = todayData ? todayData.trade_count : 0;

    const pnlEl = document.getElementById('totalPnl');
    if (pnlEl) {
      pnlEl.textContent = (todayPnl >= 0 ? '+$' : '-$') + fmt(Math.abs(todayPnl));
      pnlEl.style.color = todayPnl >= 0 ? 'var(--green)' : 'var(--red)';
    }
    setText('tradesToday', todayTrades);
    setText('winRate', Math.round(p.win_rate * 100) + '%');
  },

  renderSummaryBanner() {
    const m = this.state.metrics;
    if (m) {
      setText('sumPayouts', '$' + fmt(m.lifetime_payouts));
      setText('sumPhase', (m.current_phase || 'initiate').toUpperCase().replace('_', ' '));
    }
  },

  // ─── Render: Command Throne ───────────────────────────

  renderThrone() {
    const container = document.getElementById('fortressList');
    if (!this.state.fortresses.length) {
      container.innerHTML = `
        <div class="empty-state">
          No fortresses registered.<br>
          <button class="btn btn-primary" style="margin-top: 12px" onclick="App.createFortress()">
            Create Fortress Primus
          </button>
        </div>`;
      return;
    }

    container.innerHTML = this.state.fortresses.map(f => {
      const companies = f.companies || [];
      const allMarines = companies.flatMap(c => c.marines || []);
      const active = allMarines.filter(m => !['dormant','disabled','failed'].includes(m.status)).length;
      const dormant = allMarines.filter(m => m.status === 'dormant').length;
      const failed = allMarines.filter(m => m.status === 'failed').length;
      const disabled = allMarines.filter(m => m.status === 'disabled').length;
      return `
        <div class="fortress-card" onclick="App.viewFortress('${f.id}')">
          <div class="fortress-header">
            <span class="fortress-name">${esc(f.name)}</span>
            <span class="fortress-class">${esc(f.asset_class)}</span>
          </div>
          <div style="display:flex;gap:8px;margin-bottom:10px;font-size:11px;font-family:var(--font-mono)">
            <span style="color:var(--green)">${active} active</span>
            <span style="color:var(--text-3)">${dormant} dormant</span>
            ${failed > 0 ? `<span style="color:var(--red)">${failed} failed</span>` : ''}
            ${disabled > 0 ? `<span style="color:var(--text-3);opacity:0.5">${disabled} disabled</span>` : ''}
          </div>
          ${companies.map(c => {
            const marines = c.marines || [];
            const compActive = marines.filter(m => !['dormant','disabled','failed'].includes(m.status)).length;
            return `
              <div class="company-row">
                <span class="company-name">${esc(c.name)} <span style="color:var(--text-3);font-size:10px;margin-left:4px">${c.type || ''}</span></span>
                <span class="company-marines">
                  ${marines.length} marines
                  ${compActive > 0 ? `<span class="marine-pill active">${compActive} active</span>` : ''}
                </span>
              </div>`;
          }).join('')}
          ${companies.length === 0 ? '<div class="company-row"><span class="company-name" style="color:var(--text-3)">No companies — click to add</span></div>' : ''}
        </div>`;
    }).join('');
  },

  // ─── Render: Tactical ─────────────────────────────────

  renderTactical() {
    const container = document.getElementById('tacticalMarines');
    const marines = this.state.marines;

    if (!marines.length) {
      container.innerHTML = '<div class="empty-state">No marines registered. Add marines via the Command Throne.</div>';
      return;
    }

    container.innerHTML = marines.map(m => {
      const statusClass = ['waking','orienting','deciding','acting','reporting'].includes(m.status) ? 'active' : m.status;
      const sched = m.schedule || {};
      const schedType = sched.type || 'manual';
      const schedTag = sched.enabled
        ? `<span class="sched-tag enabled">${schedType === 'interval' ? sched.interval : schedType}</span>`
        : `<span class="sched-tag ${schedType === 'manual' ? 'manual' : 'disabled'}">${schedType}</span>`;
      const lastWake = m.last_wake ? formatTime(m.last_wake) : '—';
      return `
        <div class="marine-card" data-marine-id="${m.id}">
          <div class="marine-status-dot ${statusClass}"></div>
          <div class="marine-info">
            <div class="marine-name">${esc(m.name)}</div>
            <div class="marine-detail">${esc(m.strategy_name)} @ ${esc(m.broker_account_id || 'unassigned')}</div>
            <div class="marine-schedule">
              ${schedTag}
              <span>last wake: ${lastWake}</span>
            </div>
          </div>
          <div style="font-family: var(--font-mono); font-size: 11px; color: var(--text-2)">
            <div>${m.status}</div>
            ${m.parameters?.daily_pnl ? `<div style="color:${parseFloat(m.parameters.daily_pnl) >= 0 ? 'var(--green)' : 'var(--red)'}">${m.parameters.daily_pnl.startsWith('-') ? '' : '+'}$${fmt(Math.abs(parseFloat(m.parameters.daily_pnl)))}</div>` : ''}
            ${m.parameters?.trades_today ? `<div>${m.parameters.trades_today} trades</div>` : ''}
            ${m.parameters?.regime ? `<div>${m.parameters.regime}</div>` : ''}
          </div>
          <div class="marine-actions">
            ${m.status === 'disabled'
              ? `<button class="btn btn-sm btn-success" onclick="event.stopPropagation(); App.enableMarine('${m.id}')">Enable</button>`
              : `<button class="btn btn-sm" onclick="event.stopPropagation(); App.wakeMarine('${m.id}')">Wake</button>`}
            <button class="btn btn-sm btn-danger" onclick="event.stopPropagation(); App.disableMarine('${m.id}')">Kill</button>
          </div>
        </div>`;
    }).join('');

    // Render signal gauges (demo/placeholder)
    this.renderGauges();
  },

  renderGauges() {
    const container = document.getElementById('signalGauges');
    if (!this.state.marines.length) return;

    // Generate gauges for each active marine's indicators
    const gauges = this.state.marines
      .filter(m => m.status !== 'disabled')
      .flatMap(m => {
        const indicators = ['Momentum', 'Mean Rev', 'Volume'];
        return indicators.map(ind => ({
          label: `${m.name} ${ind}`,
          value: Math.random(), // Placeholder — will be real data from Librarium
          marine: m.name,
        }));
      });

    container.innerHTML = gauges.map(g => {
      const pct = Math.round(g.value * 100);
      let state = 'cold';
      if (pct > 80) state = 'signal';
      else if (pct > 60) state = 'hot';
      else if (pct > 35) state = 'warm';

      return `
        <div class="gauge ${state}">
          <div class="gauge-label">${esc(g.label)}</div>
          <div class="gauge-bar">
            <div class="gauge-fill" style="width: ${pct}%"></div>
          </div>
          <div class="gauge-value">${pct}%</div>
        </div>`;
    }).join('');
  },

  // ─── Render: Performance ──────────────────────────────

  renderPerformance() {
    const p = this.state.performance;
    if (!p) return;

    // Stats grid
    setText('statReturn', (p.total_pnl >= 0 ? '+$' : '-$') + fmt(Math.abs(p.total_pnl)));
    const returnEl = document.getElementById('statReturn');
    if (returnEl) returnEl.style.color = p.total_pnl >= 0 ? 'var(--green)' : 'var(--red)';
    setText('statWinRate', Math.round(p.win_rate * 100) + '%');
    setText('statProfitFactor', p.profit_factor.toFixed(2));
    setText('statDrawdown', '-$' + fmt(Math.abs(p.max_drawdown)));
    setText('statTotalTrades', p.total_trades);
    setText('statAvgWin', '+$' + fmt(p.avg_win));
    setText('statAvgLoss', '-$' + fmt(Math.abs(p.avg_loss)));
    const durationSec = Math.round((p.avg_duration_ms || 0) / 1000);
    const dMin = Math.floor(durationSec / 60);
    const dSec = durationSec % 60;
    setText('statSharpe', `${dMin}m ${dSec}s`); // Repurpose Sharpe for avg duration

    // Trade journal
    this.renderTradeJournal();

    // Equity curve — daily cumulative P&L
    this.renderEquityCurve();
  },

  renderTradeJournal() {
    const tbody = document.querySelector('#tradeJournal tbody');
    if (!tbody) return;
    const trades = this.state.trades.slice(0, 50);
    if (!trades.length) {
      tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;color:var(--text-3)">No trades synced yet</td></tr>';
      return;
    }
    tbody.innerHTML = trades.map(t => {
      const pnlColor = t.pnl >= 0 ? 'var(--green)' : 'var(--red)';
      const pnlStr = (t.pnl >= 0 ? '+' : '') + '$' + Math.abs(t.pnl).toFixed(2);
      const meta = t.metadata || {};
      const signal = meta.signal_type || '—';
      const regime = meta.regime || '';
      const exitReason = meta.exit_reason || '';
      const dur = Math.round((t.duration_ms || 0) / 1000);
      const durStr = dur >= 60 ? Math.floor(dur / 60) + 'm ' + (dur % 60) + 's' : dur + 's';
      return `<tr>
        <td>${formatDate(t.exit_time)} ${formatTime(t.exit_time)}</td>
        <td><span style="color:var(--cyan)">${esc(signal)}</span></td>
        <td>${t.side} ${t.quantity}x ${esc(t.symbol)}</td>
        <td>$${Number(t.entry_price).toFixed(2)}</td>
        <td>$${Number(t.exit_price).toFixed(2)}</td>
        <td style="color:${pnlColor};font-weight:600">${pnlStr}</td>
        <td>${esc(exitReason)}</td>
        <td>${durStr}</td>
      </tr>`;
    }).join('');
  },

  renderEquityCurve() {
    const container = document.getElementById('equityChart');
    if (!container) return;
    const p = this.state.performance;
    if (!p || !p.daily_pnl || !p.daily_pnl.length) return;

    // Sort daily P&L by date
    const days = [...p.daily_pnl].sort((a, b) => a.date.localeCompare(b.date));

    // Build cumulative equity
    let cum = 0;
    const points = days.map(d => {
      cum += d.pnl;
      return { date: d.date, cum, pnl: d.pnl, trades: d.trade_count };
    });

    // Simple SVG chart
    const w = 800, h = 200, pad = 40;
    const maxY = Math.max(...points.map(p => p.cum), 0);
    const minY = Math.min(...points.map(p => p.cum), 0);
    const rangeY = maxY - minY || 1;

    const xStep = (w - pad * 2) / Math.max(points.length - 1, 1);
    const toX = i => pad + i * xStep;
    const toY = v => pad + (1 - (v - minY) / rangeY) * (h - pad * 2);

    const pathD = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${toX(i).toFixed(1)} ${toY(p.cum).toFixed(1)}`).join(' ');
    const areaD = pathD + ` L ${toX(points.length - 1).toFixed(1)} ${toY(0).toFixed(1)} L ${toX(0).toFixed(1)} ${toY(0).toFixed(1)} Z`;

    // Calendar grid below
    const calendar = this.renderDailyCalendar(days);

    container.innerHTML = `
      <svg viewBox="0 0 ${w} ${h}" style="width:100%;height:auto;max-height:220px">
        <defs>
          <linearGradient id="eqGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--green)" stop-opacity="0.3"/>
            <stop offset="100%" stop-color="var(--green)" stop-opacity="0"/>
          </linearGradient>
        </defs>
        <!-- Zero line -->
        <line x1="${pad}" y1="${toY(0)}" x2="${w - pad}" y2="${toY(0)}" stroke="var(--border)" stroke-dasharray="4"/>
        <!-- Area -->
        <path d="${areaD}" fill="url(#eqGrad)"/>
        <!-- Line -->
        <path d="${pathD}" fill="none" stroke="var(--green)" stroke-width="2"/>
        <!-- Points -->
        ${points.map((p, i) => `<circle cx="${toX(i)}" cy="${toY(p.cum)}" r="3" fill="var(--green)" stroke="var(--bg)" stroke-width="1.5">
          <title>${p.date}: ${p.pnl >= 0 ? '+' : ''}$${p.pnl.toFixed(0)} (${p.trades} trades)\nCumulative: $${p.cum.toFixed(0)}</title>
        </circle>`).join('')}
        <!-- Labels -->
        <text x="${pad}" y="${h - 5}" fill="var(--text-3)" font-size="10">${points[0]?.date.slice(5) || ''}</text>
        <text x="${w - pad}" y="${h - 5}" fill="var(--text-3)" font-size="10" text-anchor="end">${points[points.length - 1]?.date.slice(5) || ''}</text>
        <text x="${pad - 5}" y="${toY(maxY) + 4}" fill="var(--text-3)" font-size="10" text-anchor="end">$${fmt(maxY)}</text>
        <text x="${pad - 5}" y="${toY(minY) + 4}" fill="var(--text-3)" font-size="10" text-anchor="end">$${fmt(minY)}</text>
      </svg>
      ${calendar}
    `;
  },

  renderDailyCalendar(days) {
    if (!days || !days.length) return '';
    const rows = days.map(d => {
      const pnlColor = d.pnl >= 0 ? 'var(--green)' : 'var(--red)';
      const pnlStr = (d.pnl >= 0 ? '+' : '') + '$' + Math.abs(d.pnl).toFixed(0);
      const dow = new Date(d.date + 'T12:00:00').toLocaleDateString('en-US', { weekday: 'short' });
      return `<div class="daily-pnl-cell" style="border-left: 3px solid ${pnlColor}; padding: 4px 8px; margin: 2px 0;">
        <span style="color:var(--text-3);width:70px;display:inline-block">${d.date.slice(5)} ${dow}</span>
        <span style="color:${pnlColor};font-weight:600;width:80px;display:inline-block;text-align:right">${pnlStr}</span>
        <span style="color:var(--text-3);margin-left:8px">${d.trades} trade${d.trades !== 1 ? 's' : ''}</span>
      </div>`;
    }).join('');
    return `<div style="margin-top:12px;max-height:200px;overflow-y:auto;font-size:12px;font-family:var(--font-mono)">${rows}</div>`;
  },

  // ─── Render: Event Feed ───────────────────────────────

  renderEventFeed() {
    const container = document.getElementById('eventFeed');
    const countEl = document.getElementById('eventCount');
    countEl.textContent = this.state.events.length;

    if (!this.state.events.length) return;

    const icons = {
      wake: '▲', sleep: '▼', failed: '✕', started: '●',
      marine_scheduled: '◷', stopped: '■',
    };
    const eventColors = {
      wake: 'var(--green)', sleep: 'var(--blue)', failed: 'var(--red)',
      started: 'var(--gold)', stopped: 'var(--text-3)', marine_scheduled: 'var(--amber)',
    };

    container.innerHTML = this.state.events.slice(0, 50).map(e => `
      <div class="event-item">
        <span class="event-time">${formatTime(e.timestamp)}</span>
        <span class="event-icon" style="color:${eventColors[e.event] || 'var(--text-2)'}">${icons[e.event] || '•'}</span>
        <span class="event-msg">${esc(e.service)}.${esc(e.event)}</span>
        ${e.marine_id ? `<span class="event-marine">${esc(e.marine_id)}</span>` : ''}
      </div>`
    ).join('');
  },

  // ─── Actions ──────────────────────────────────────────

  async wakeMarine(id) {
    try {
      await this.api(`/marines/${id}/wake`, { method: 'POST' });
    } catch (e) {
      this.flash(`Failed to wake marine: ${e.message}`, 'error');
    }
  },

  async enableMarine(id) {
    try {
      await this.api(`/marines/${id}/enable`, { method: 'POST' });
      this.loadData();
    } catch (e) {
      this.flash(`Failed to enable marine: ${e.message}`, 'error');
    }
  },

  async disableMarine(id) {
    try {
      await this.api(`/marines/${id}/disable`, { method: 'POST' });
      this.loadData();
    } catch (e) {
      this.flash(`Failed to disable marine: ${e.message}`, 'error');
    }
  },

  async killSwitch() {
    if (!confirm('ACTIVATE KILL SWITCH?\n\nThis will immediately disable ALL marines across the entire Imperium.')) return;
    try {
      await this.api('/kill-switch/imperium', { method: 'POST' });
      this.loadData();
      this.flash('KILL SWITCH ACTIVATED — All marines disabled', 'error');
    } catch (e) {
      this.flash(`Kill switch failed: ${e.message}`, 'error');
    }
  },

  createFortress() {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Create Fortress</h3>
      <form onsubmit="App.submitFortress(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>ID</label>
          <input type="text" id="newFortressId" class="input" placeholder="fortress-primus" required>
        </div>
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newFortressName" class="input" placeholder="Fortress Primus" required>
        </div>
        <div class="form-row">
          <label>Asset Class</label>
          <select id="newFortressClass" class="input">
            <option value="equities">Equities</option>
            <option value="futures">Futures</option>
            <option value="options">Options</option>
            <option value="etf">ETFs</option>
            <option value="fixed_income">Fixed Income</option>
            <option value="crypto">Crypto</option>
            <option value="multi_asset">Multi-Asset</option>
          </select>
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Create</button>
      </form>
    `);
  },

  async submitFortress(e) {
    e.preventDefault();
    try {
      await this.api('/fortresses', {
        method: 'POST',
        body: {
          id: document.getElementById('newFortressId').value,
          name: document.getElementById('newFortressName').value,
          asset_class: document.getElementById('newFortressClass').value,
        },
      });
      this.closeModal();
      this.loadData();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  viewFortress(id) {
    const f = this.state.fortresses.find(f => f.id === id);
    if (!f) return;
    const companies = f.companies || [];

    this.openModal(`
      <h3 style="margin-bottom: 4px">${esc(f.name)}</h3>
      <div style="color: var(--gold); font-size: 11px; font-family: var(--font-mono); margin-bottom: 16px">
        ${esc(f.asset_class.toUpperCase())}
      </div>
      <div style="display:flex;gap:8px;margin-bottom:16px">
        <button class="btn btn-sm" onclick="App.addCompany('${f.id}')">+ Add Company</button>
        <button class="btn btn-sm" onclick="App.addMarine('${f.id}')">+ Add Marine</button>
      </div>
      ${companies.length === 0
        ? '<div class="empty-state">No companies yet</div>'
        : companies.map(c => {
            const marines = c.marines || [];
            return `
              <div style="margin-bottom: 12px">
                <div style="font-weight:600; margin-bottom:6px">${esc(c.name)}
                  <span style="color:var(--text-3);font-size:11px;margin-left:8px">${c.type}</span>
                </div>
                ${marines.length === 0
                  ? '<div style="color:var(--text-3);font-size:12px;padding:4px 0">No marines assigned</div>'
                  : marines.map(m => `
                    <div class="marine-card" style="margin-bottom:4px" data-marine-id="${m.id}">
                      <div class="marine-status-dot ${m.status}"></div>
                      <div class="marine-info">
                        <div class="marine-name">${esc(m.name)}</div>
                        <div class="marine-detail">${esc(m.strategy_name)} @ ${esc(m.broker_account_id || '—')}</div>
                      </div>
                      <div style="font-family:var(--font-mono);font-size:11px;color:var(--text-2)">${m.status}</div>
                    </div>`).join('')}
              </div>`;
          }).join('')}
    `);
  },

  addCompany(fortressId) {
    this.closeModal();
    this.openModal(`
      <h3 style="margin-bottom: 16px">Add Company to ${fortressId}</h3>
      <form onsubmit="App.submitCompany(event, '${fortressId}')" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>ID</label>
          <input type="text" id="newCompanyId" class="input" placeholder="first-company" required>
        </div>
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newCompanyName" class="input" placeholder="1st Company (Veterans)" required>
        </div>
        <div class="form-row">
          <label>Type</label>
          <select id="newCompanyType" class="input">
            <option value="veteran">Veteran (Live Primary)</option>
            <option value="battle">Battle (Live Secondary)</option>
            <option value="reserve">Reserve (Paper/Staging)</option>
            <option value="scout">Scout (Experimental)</option>
          </select>
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Create Company</button>
      </form>
    `);
  },

  async submitCompany(e, fortressId) {
    e.preventDefault();
    try {
      await this.api('/companies', {
        method: 'POST',
        body: {
          id: document.getElementById('newCompanyId').value,
          name: document.getElementById('newCompanyName').value,
          fortress_id: fortressId,
          type: document.getElementById('newCompanyType').value,
        },
      });
      this.closeModal();
      this.loadData();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  addMarine(fortressId) {
    const fortress = this.state.fortresses.find(f => f.id === fortressId);
    const companies = fortress?.companies || [];

    this.closeModal();
    this.openModal(`
      <h3 style="margin-bottom: 16px">Register Marine</h3>
      <form onsubmit="App.submitMarine(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>ID</label>
          <input type="text" id="newMarineId" class="input" placeholder="alpha-1" required>
        </div>
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newMarineName" class="input" placeholder="Marine Alpha-1" required>
        </div>
        <div class="form-row">
          <label>Company</label>
          <select id="newMarineCompany" class="input" required>
            ${companies.map(c => `<option value="${c.id}">${esc(c.name)}</option>`).join('')}
          </select>
        </div>
        <div class="form-row">
          <label>Strategy Name</label>
          <input type="text" id="newMarineStrategy" class="input" placeholder="es-momentum" required>
        </div>
        <div class="form-row">
          <label>Broker Account</label>
          <input type="text" id="newMarineBroker" class="input" placeholder="APEX-001">
        </div>
        <div class="form-row">
          <label>Runner Type</label>
          <select id="newMarineRunner" class="input">
            <option value="process">Process (local Python script)</option>
            <option value="remote">Remote (connect to running strategy)</option>
            <option value="docker">Docker (container)</option>
          </select>
        </div>
        <div class="form-row">
          <label>Schedule</label>
          <select id="newMarineSchedule" class="input">
            <option value="manual">Manual (on-demand)</option>
            <option value="30s">Every 30 seconds</option>
            <option value="1m">Every 1 minute</option>
            <option value="5m">Every 5 minutes</option>
          </select>
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Register Marine</button>
      </form>
    `);
  },

  async submitMarine(e) {
    e.preventDefault();
    const schedMap = { manual: 'manual', '30s': 'interval', '1m': 'interval', '5m': 'interval' };
    const schedVal = document.getElementById('newMarineSchedule').value;
    try {
      await this.api('/marines', {
        method: 'POST',
        body: {
          id: document.getElementById('newMarineId').value,
          name: document.getElementById('newMarineName').value,
          company_id: document.getElementById('newMarineCompany').value,
          strategy_name: document.getElementById('newMarineStrategy').value,
          broker_account_id: document.getElementById('newMarineBroker').value,
          runner_type: document.getElementById('newMarineRunner').value,
          schedule: {
            type: schedMap[schedVal] || 'manual',
            interval: schedVal !== 'manual' ? schedVal : '',
            enabled: schedVal !== 'manual',
          },
        },
      });
      this.closeModal();
      this.loadData();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  submitBacktest(e) {
    e.preventDefault();
    this.flash('Forge not connected yet — wire up astartes-futures backtester', 'info');
  },

  // ─── Council ──────────────────────────────────────────

  async loadCouncil() {
    try {
      const [roadmap, accounts, payouts, budget, allocations, advice, metrics, goals, expenses, billing, budgets, propFirms] = await Promise.all([
        this.api('/council/roadmap'),
        this.api('/council/accounts'),
        this.api('/council/payouts'),
        this.api('/council/budget'),
        this.api('/council/allocations'),
        this.api('/council/withdrawal-advice'),
        this.api('/council/metrics'),
        this.api('/council/goals'),
        this.api('/council/expenses'),
        this.api('/council/billing'),
        this.api('/council/budgets').catch(() => null),
        this.api('/council/prop-firms').catch(() => null),
      ]);
      if (propFirms) {
        this.state.propFirms = propFirms.firms || [];
        this.state.taxWithholdPct = propFirms.tax_withhold_pct || 0.30;
        this.state.fireflyAccounts = propFirms.firefly_asset_accounts || [];
      }
      this.state.roadmap = roadmap;
      this.state.accounts = accounts || [];
      this.state.payouts = payouts || [];
      this.state.budget = budget;
      this.state.allocations = allocations || [];
      this.state.withdrawalAdvice = advice || [];
      this.state.metrics = metrics;
      this.state.goals = goals || [];
      this.state.expenses = expenses || [];
      this.state.billing = billing;
      this.state.unifiedBudgets = budgets || [];
      this.renderCouncil();
    } catch (e) {
      // Primarch might not be running
    }
  },

  renderCouncil() {
    this.renderPhaseTracker();
    this.renderAccounts();
    this.renderWithdrawalAdvice();
    this.renderPayouts();
    this.renderBusinessMetrics();
    this.renderAllocations();
    this.renderGoals();
    this.renderBilling();
    this.renderUnifiedBudgets();
    this.renderSummaryBanner();
    this.renderPropDesk();
  },

  renderPhaseTracker() {
    const rm = this.state.roadmap;
    if (!rm) return;
    const el = document.getElementById('phaseTracker');
    const label = document.getElementById('currentPhaseLabel');

    const tracks = rm.strategy_tracks || [];

    // If we have strategy tracks, render per-strategy progression
    if (tracks.length > 0) {
      label.textContent = tracks.some(t => t.rubicon_date) ? 'ASTARTES' : 'ASPIRANT';

      const rankOrder = ['initiate', 'neophyte', 'astartes', 'veteran', 'captain', 'chapter_master'];
      const rankLabels = {
        initiate: 'Initiate', neophyte: 'Neophyte', astartes: 'Astartes',
        veteran: 'Veteran', captain: 'Captain', chapter_master: 'Chapter Master'
      };
      const rankColors = {
        initiate: 'var(--text-3)', neophyte: 'var(--accent)', astartes: 'var(--green)',
        veteran: 'var(--gold)', captain: 'var(--gold)', chapter_master: 'var(--gold)'
      };

      el.innerHTML = tracks.map(t => {
        const rankIdx = rankOrder.indexOf(t.current_rank);
        const crossedRubicon = !!t.rubicon_date;
        const m = t.metrics || {};
        const winRate = m.win_rate ? m.win_rate.toFixed(0) + '%' : '—';
        const profitDays = m.profitable_days || 0;
        const totalDays = m.total_trading_days || 0;

        return `
          <div class="strategy-track ${crossedRubicon ? 'rubicon-crossed' : ''}">
            <div class="track-header">
              <span class="track-name">${esc(t.name)}</span>
              <span class="track-rank" style="color:${rankColors[t.current_rank] || 'var(--text-3)'}">
                ${rankLabels[t.current_rank] || t.current_rank}
              </span>
            </div>

            <div class="track-progression">
              ${rankOrder.map((rank, i) => {
                const isCurrent = rank === t.current_rank;
                const isPast = i < rankIdx;
                const isRubicon = rank === 'astartes';
                let cls = isPast ? 'completed' : isCurrent ? 'active' : 'locked';
                return `<div class="track-rank-pip ${cls} ${isRubicon ? 'rubicon-pip' : ''}">
                  <div class="pip-dot"></div>
                  <span class="pip-label">${isRubicon ? 'RUBICON' : rankLabels[rank]}</span>
                </div>`;
              }).join('<div class="pip-line"></div>')}
            </div>

            ${crossedRubicon ? `
              <div class="rubicon-banner">
                CROSSED THE RUBICON — ${t.rubicon_date}
              </div>` : ''}

            <div class="track-metrics">
              <span>${profitDays}/${totalDays} profit days</span>
              <span>Win rate: ${winRate}</span>
              ${m.accounts_funded ? `<span>${m.accounts_funded} funded</span>` : ''}
              ${m.accounts_blown ? `<span style="color:var(--red)">${m.accounts_blown} blown</span>` : ''}
              ${m.total_payouts ? `<span style="color:var(--green)">$${fmt(m.total_payouts)} paid out</span>` : ''}
            </div>
          </div>`;
      }).join('');
      return;
    }

    // Fallback: legacy phase tracker
    label.textContent = (rm.current_phase || 'initiate').toUpperCase().replace('_', ' ');
    el.innerHTML = rm.phases.map(p => {
      let cls = 'locked';
      if (p.active) cls = 'active';
      else if (p.completed_at) cls = 'completed';
      else {
        const phaseOrder = ['initiate', 'neophyte', 'battle_brother', 'veteran', 'captain', 'chapter_master'];
        const currentIdx = phaseOrder.indexOf(rm.current_phase);
        const thisIdx = phaseOrder.indexOf(p.phase);
        if (thisIdx < currentIdx) cls = 'completed';
      }
      const milestones = (p.milestones || []).slice(0, 3);
      return `
        <div class="phase-card ${cls}">
          ${cls === 'completed' ? '<span class="phase-check">✓</span>' : ''}
          <div class="phase-rank">${esc(p.title)}</div>
          <div class="phase-name">${esc(p.name)}</div>
          <div class="phase-desc">${esc(p.description).substring(0, 80)}${p.description.length > 80 ? '...' : ''}</div>
          ${milestones.length > 0 ? `
            <div class="phase-milestones">
              ${milestones.map(m => `
                <div class="phase-milestone">
                  <span class="phase-milestone-dot ${m.completed ? 'done' : ''}"></span>
                  <span>${esc(m.name)}</span>
                </div>
              `).join('')}
            </div>` : ''}
        </div>`;
    }).join('');
  },

  renderAccounts() {
    const el = document.getElementById('accountList');
    if (!this.state.accounts.length) {
      el.innerHTML = '<div class="empty-state">No accounts yet. Add your trading accounts to start tracking.</div>';
      return;
    }

    const phase = a => a.account_phase || (a.status === 'blown' ? 'blown' : (a.type === 'paper' ? 'paper' : 'combine'));
    const isFunded = a => ['fxt', 'live'].includes(phase(a)) && a.status === 'active';
    const isPaper = a => a.type === 'paper';
    const isBlown = a => phase(a) === 'blown' || a.status === 'blown';
    const isEval = a => !isFunded(a) && !isPaper(a) && !isBlown(a) && a.status === 'active';

    const groups = [
      { key: 'funded', label: 'Funded', color: 'var(--green)', accounts: this.state.accounts.filter(isFunded) },
      { key: 'eval', label: 'Evaluation', color: 'var(--accent)', accounts: this.state.accounts.filter(isEval) },
      { key: 'paper', label: 'Paper / Sim', color: 'var(--text-3)', accounts: this.state.accounts.filter(isPaper) },
      { key: 'blown', label: 'Blown', color: 'var(--red)', accounts: this.state.accounts.filter(isBlown) },
    ].filter(g => g.accounts.length > 0);

    const renderCard = (a, dimBalance) => {
      const pnlClass = a.total_pnl >= 0 ? 'positive' : 'negative';
      const balStyle = dimBalance ? ' style="color:var(--text-3)"' : '';
      return `
        <div class="account-card">
          <span class="account-type-badge ${a.type}">${a.type}</span>
          <div class="account-info">
            <div class="account-name">${esc(a.name)}</div>
            <div class="account-detail">${esc(a.broker)} · ${(a.instruments || []).join(', ') || 'N/A'} · ${a.payout_count} payouts</div>
          </div>
          <div class="account-balance">
            <div class="account-balance-value"${balStyle}>$${fmt(a.current_balance)}</div>
            <div class="account-balance-pnl ${pnlClass}">${a.total_pnl >= 0 ? '+' : ''}$${fmt(a.total_pnl)} · ${Math.round(a.profit_split * 100)}% split</div>
          </div>
        </div>`;
    };

    el.innerHTML = groups.map(g => {
      const dimBalance = g.key !== 'funded';
      const groupTotal = g.accounts.reduce((s, a) => s + a.current_balance, 0);
      const cashLabel = g.key === 'funded' ? '' : '<span style="font-size:9px;color:var(--text-3);margin-left:6px">NO CASH VALUE</span>';
      return `
        <div class="account-group">
          <div class="account-group-header" style="border-left:3px solid ${g.color}">
            <span style="color:${g.color};font-size:10px;text-transform:uppercase;letter-spacing:1px;font-weight:700">${g.label}</span>
            <span style="font-size:11px;color:var(--text-3)">${g.accounts.length} acct${g.accounts.length !== 1 ? 's' : ''}${g.key === 'funded' ? ' · $' + fmt(groupTotal) : ''}${cashLabel}</span>
          </div>
          ${g.accounts.map(a => renderCard(a, dimBalance)).join('')}
        </div>`;
    }).join('');
  },

  renderWithdrawalAdvice() {
    const el = document.getElementById('withdrawalAdvice');
    if (!this.state.withdrawalAdvice.length) {
      el.innerHTML = '<div class="empty-state">Add accounts to get withdrawal advice</div>';
      return;
    }
    el.innerHTML = this.state.withdrawalAdvice.map(w => `
      <div class="withdrawal-card urgency-${w.urgency}">
        <div class="withdrawal-header">
          <span class="withdrawal-account">${esc(w.account_name)}</span>
          <span class="withdrawal-urgency ${w.urgency}">${w.urgency}</span>
        </div>
        <div class="withdrawal-reason">${esc(w.reason)}</div>
        ${w.recommended_amount > 0 ? `
          <div class="withdrawal-amount">Withdraw: $${fmt(w.recommended_amount)}</div>
          <div class="withdrawal-splits">
            ${(w.allocations || []).map(a => `
              <span class="withdrawal-split">${a.category}: $${fmt(a.amount)}</span>
            `).join('')}
          </div>
        ` : ''}
      </div>
    `).join('');
  },

  renderPayouts() {
    const el = document.getElementById('payoutList');
    if (!this.state.payouts.length) {
      el.innerHTML = '<div class="empty-state">No payouts recorded yet</div>';
      return;
    }
    el.innerHTML = this.state.payouts.map(p => `
      <div class="payout-item">
        <span class="payout-date">${formatDate(p.requested_at)}</span>
        <span class="payout-account">${esc(p.account_id)}</span>
        <span class="payout-amount">+$${fmt(p.net_amount)}</span>
        <span class="payout-dest">→ ${p.destination}</span>
      </div>
    `).join('');
  },

  renderBusinessMetrics() {
    const m = this.state.metrics;
    if (!m) return;
    // Funded P&L (real cash) vs total
    const fundedPnl = m.funded_pnl || 0;
    const simPnl = m.sim_pnl || 0;
    const fundedCap = m.funded_capital || 0;

    setText('bizLifetimePnl', '$' + fmt(m.lifetime_pnl));
    setText('bizLifetimePayouts', '$' + fmt(m.lifetime_payouts));
    setText('bizMonthlyNet', '$' + fmt(m.monthly_net_income));
    setText('bizPersonalValue', '$' + fmt(m.personal_account_value));
    setText('bizGoalProgress', Math.round(m.goal_progress * 100) + '%');
    setText('bizPhase', (m.current_phase || '—').replace('_', ' '));
    setText('bizProfitDays', m.profitable_days + '/' + m.total_trading_days);
    setText('bizBlown', m.accounts_blown);

    // Inject funded/sim breakdown below lifetime P&L
    const fundedEl = document.getElementById('bizFundedBreakdown');
    if (fundedEl) {
      fundedEl.innerHTML = `
        <span style="color:var(--green);font-size:10px">Funded: ${fundedPnl >= 0 ? '+' : ''}$${fmt(fundedPnl)}</span>
        <span style="color:var(--text-3);font-size:10px">Sim: ${simPnl >= 0 ? '+' : ''}$${fmt(simPnl)}</span>`;
    }
    const fundedCapEl = document.getElementById('bizFundedCapital');
    if (fundedCapEl) {
      fundedCapEl.textContent = '$' + fmt(fundedCap);
    }
    const acctBreakdown = document.getElementById('bizAcctBreakdown');
    if (acctBreakdown) {
      acctBreakdown.innerHTML = `
        <span style="color:var(--green);font-size:10px">${m.accounts_funded || 0} funded</span>
        <span style="color:var(--accent);font-size:10px">${m.accounts_in_combine || 0} combine</span>`;
    }
  },

  renderAllocations() {
    const el = document.getElementById('allocationBars');
    const allocs = this.state.allocations;
    if (!allocs.length) return;
    const colors = { bills: '#ef4444', trading_capital: '#60a5fa', taxes: '#fbbf24', savings: '#2dd4a0', personal: '#a78bfa' };
    el.innerHTML = allocs.map(a => `
      <div class="alloc-row">
        <span class="alloc-label">${esc(a.category)}</span>
        <div class="alloc-bar-wrap">
          <div class="alloc-bar-fill" style="width:${a.percentage}%; background:${colors[a.category] || 'var(--text-2)'}"></div>
        </div>
        <span class="alloc-pct">${a.percentage}%</span>
      </div>
    `).join('');
  },

  renderGoals() {
    const el = document.getElementById('goalList');
    if (!this.state.goals.length) {
      el.innerHTML = '<div class="empty-state">No goals yet — add your first target</div>';
      return;
    }
    // Sort by priority (1 = highest), then by completion status
    // Firefly piggy banks use percentage field, store goals use status
    const sorted = [...this.state.goals].sort((a, b) => {
      const aComplete = a.status === 'completed' || a.percentage >= 100;
      const bComplete = b.status === 'completed' || b.percentage >= 100;
      if (aComplete && !bComplete) return 1;
      if (bComplete && !aComplete) return -1;
      return (a.priority || 3) - (b.priority || 3);
    });
    const categoryIcons = {
      home_improvement: '🏠', vehicle: '🚗', savings: '💰',
      trading: '📈', debt: '💳', lifestyle: '🎯', business: '🏢',
    };
    el.innerHTML = sorted.map(g => {
      const isFirefly = !!g.source;
      const pct = g.percentage || (g.target_amount > 0 ? Math.min((g.current_amount / g.target_amount) * 100, 100) : 0);
      const icon = g.icon || categoryIcons[g.category] || '🎯';
      const isComplete = g.status === 'completed' || pct >= 100;
      const priorityDots = !isFirefly ? Array.from({length: 5}, (_, i) =>
        `<span class="goal-priority-dot ${i < (6 - (g.priority || 3)) ? 'filled' : ''}"></span>`
      ).join('') : '';
      const notes = g.notes || g.description || '';
      return `
        <div class="goal-card ${isComplete ? 'completed' : (g.status || 'active')}">
          <div class="goal-header">
            <span class="goal-name"><span class="goal-icon">${icon}</span> ${esc(g.name)}</span>
            ${isFirefly ? '<span class="expense-autopay" style="background:var(--blue);font-size:9px">CFO</span>' : ''}
            <span class="goal-category ${g.category || ''}">${(g.category || '').replace('_', ' ')}</span>
          </div>
          ${notes ? `<div style="font-size:11px;color:var(--text-2);margin-bottom:6px">${esc(notes)}</div>` : ''}
          <div class="goal-progress">
            <div class="goal-progress-bar">
              <div class="goal-progress-fill" style="width:${pct}%"></div>
            </div>
          </div>
          <div class="goal-amounts">
            <span class="current">$${fmt(g.current_amount)}</span>
            <span>$${fmt(g.target_amount)}</span>
          </div>
          <div class="goal-footer">
            ${priorityDots ? `<div class="goal-priority" title="Priority">${priorityDots}</div>` : ''}
            ${g.target_date ? `<span style="font-size:10px;color:var(--text-2)">Target: ${g.target_date}</span>` : ''}
            ${g.payouts_needed > 0 ? `<span class="goal-payouts-needed">${g.payouts_needed} payouts away</span>` : ''}
            ${isComplete ? '<span style="color:var(--green)">COMPLETED</span>' : ''}
            <div class="goal-actions">
              ${!isFirefly && g.status === 'active' ? `<button class="btn btn-sm" onclick="event.stopPropagation(); App.contributeGoal('${g.id}', '${esc(g.name)}')">+ Fund</button>` : ''}
            </div>
          </div>
        </div>`;
    }).join('');
  },

  renderBilling() {
    const b = this.state.billing;
    if (b) {
      setText('billingLife', '$' + fmt(b.life_expenses || 0));
      setText('billingSystem', '$' + fmt(b.system_expenses || 0));
      setText('billingTotal', '$' + fmt(b.total_expenses));
      const coverage = Math.round((b.trading_coverage || 0) * 100);
      setText('billingCoverage', coverage + '%');

      // Coverage bar — how much of real life is funded by trading
      const barEl = document.getElementById('billingCoverageBar');
      if (barEl && (b.life_expenses || 0) > 0) {
        const coverColor = coverage >= 100 ? 'var(--green)' : coverage >= 50 ? 'var(--gold)' : 'var(--red)';
        barEl.innerHTML = `
          <div style="font-size:10px;color:var(--text-3);margin-bottom:3px;text-transform:uppercase;letter-spacing:0.8px">Bot funding real life</div>
          <div class="progress-bar"><div class="progress-fill" style="width:${Math.min(coverage,100)}%;background:${coverColor}"></div></div>`;
      }
    }

    // Merge unified bills from billing API + manual expenses
    const bills = (b && b.bills) || [];
    const manual = this.state.expenses || [];
    const lifeBills = bills.filter(e => e.kind === 'life');
    const systemBills = bills.filter(e => e.kind === 'system');
    // Manual expenses that aren't already in the unified list
    const manualOnly = manual.filter(e => !e.source);

    const el = document.getElementById('expenseList');
    if (!lifeBills.length && !systemBills.length && !manualOnly.length) {
      el.innerHTML = '<div class="empty-state">Connect Monarch to see real-life bills</div>';
      return;
    }

    const renderBill = (e, isManual) => {
      const sourceTag = e.source === 'family'
        ? '<span class="expense-autopay" style="background:var(--accent)">Monarch</span>'
        : e.source === 'personal'
          ? '<span class="expense-autopay" style="background:var(--blue)">CFO</span>'
          : '';
      return `
        <div class="expense-item">
          <span class="expense-name">${esc(e.name)}</span>
          <span class="expense-category">${esc(e.category || '')}</span>
          <span class="expense-amount">$${fmt(e.amount)}</span>
          <span class="expense-freq">${e.repeat_freq || e.frequency || 'monthly'}</span>
          ${sourceTag}
          ${isManual ? `<div class="expense-actions">
            <button class="btn btn-sm" onclick="event.stopPropagation(); App.payExpense('${e.id}', '${esc(e.name)}', ${e.amount})" title="Record payment">Pay</button>
          </div>` : ''}
        </div>`;
    };

    let html = '';
    if (lifeBills.length) {
      html += `<div class="account-group">
        <div class="account-group-header" style="border-left:3px solid var(--accent)">
          <span style="color:var(--accent);font-size:10px;text-transform:uppercase;letter-spacing:1px;font-weight:700">Life Bills</span>
          <span style="font-size:11px;color:var(--text-3)">${lifeBills.length} bills · $${fmt(lifeBills.reduce((s,b) => s + b.amount, 0))}/mo</span>
        </div>
        ${lifeBills.map(e => renderBill(e, false)).join('')}
      </div>`;
    }
    if (systemBills.length || manualOnly.length) {
      const allSystem = [...systemBills, ...manualOnly];
      html += `<div class="account-group">
        <div class="account-group-header" style="border-left:3px solid var(--gold)">
          <span style="color:var(--gold);font-size:10px;text-transform:uppercase;letter-spacing:1px;font-weight:700">System Costs</span>
          <span style="font-size:11px;color:var(--text-3)">${allSystem.length} items · $${fmt(allSystem.reduce((s,b) => s + b.amount, 0))}/mo</span>
        </div>
        ${allSystem.map(e => renderBill(e, !e.source)).join('')}
      </div>`;
    }
    el.innerHTML = html;
  },

  renderUnifiedBudgets() {
    const el = document.getElementById('unifiedBudgetList');
    if (!el) return;
    const budgets = this.state.unifiedBudgets || [];
    if (!budgets.length) {
      el.innerHTML = '<div class="empty-state">No budgets — connect Monarch or CFO Engine</div>';
      return;
    }

    const sourceLabel = { personal: 'CFO', family: 'Monarch' };
    const sourceColor = { personal: 'var(--blue)', family: 'var(--accent)' };

    // Group by source
    const personal = budgets.filter(b => b.source === 'personal');
    const family = budgets.filter(b => b.source === 'family');
    const groups = [];
    if (family.length) groups.push({ label: 'Family (Monarch)', color: 'var(--accent)', items: family });
    if (personal.length) groups.push({ label: 'Personal (CFO)', color: 'var(--blue)', items: personal });

    el.innerHTML = groups.map(g => `
      <div class="budget-group">
        <div class="account-group-header" style="border-left:3px solid ${g.color}">
          <span style="color:${g.color};font-size:10px;text-transform:uppercase;letter-spacing:1px;font-weight:700">${g.label}</span>
          <span style="font-size:11px;color:var(--text-3)">${g.items.length} categories</span>
        </div>
        ${g.items.map(b => {
          const pct = b.budgeted > 0 ? Math.min((b.spent / b.budgeted) * 100, 100) : 0;
          const remaining = b.budgeted - b.spent;
          const overBudget = remaining < 0;
          const barColor = overBudget ? 'var(--red)' : pct > 80 ? 'var(--gold)' : 'var(--green)';
          return `
            <div class="budget-row">
              <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:3px">
                <span style="font-size:12px;font-weight:600">${esc(b.category)}</span>
                <span style="font-size:11px;font-family:var(--font-mono);color:${overBudget ? 'var(--red)' : 'var(--text-2)'}">$${fmt(b.spent)} / $${fmt(b.budgeted)}</span>
              </div>
              <div class="progress-bar"><div class="progress-fill" style="width:${pct}%;background:${barColor}"></div></div>
            </div>`;
        }).join('')}
      </div>
    `).join('');
  },

  addGoal() {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Add Goal</h3>
      <form onsubmit="App.submitGoal(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newGoalName" class="input" placeholder="Corvette, Garage, Porch..." required>
        </div>
        <div class="form-row">
          <label>Description (optional)</label>
          <input type="text" id="newGoalDesc" class="input" placeholder="Details about this goal">
        </div>
        <div class="form-row two-col">
          <div>
            <label>Category</label>
            <select id="newGoalCat" class="input">
              <option value="home_improvement">Home Improvement</option>
              <option value="vehicle">Vehicle</option>
              <option value="savings">Savings</option>
              <option value="trading">Trading</option>
              <option value="debt">Debt Payoff</option>
              <option value="lifestyle">Lifestyle</option>
              <option value="business">Business</option>
            </select>
          </div>
          <div>
            <label>Priority (1=highest)</label>
            <select id="newGoalPriority" class="input">
              <option value="1">1 — Critical</option>
              <option value="2">2 — High</option>
              <option value="3" selected>3 — Medium</option>
              <option value="4">4 — Low</option>
              <option value="5">5 — Someday</option>
            </select>
          </div>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Target Amount ($)</label>
            <input type="number" id="newGoalTarget" class="input" placeholder="15000" required>
          </div>
          <div>
            <label>Already Saved ($)</label>
            <input type="number" id="newGoalCurrent" class="input" value="0">
          </div>
        </div>
        <div class="form-row">
          <label>Target Date (optional)</label>
          <input type="date" id="newGoalDate" class="input">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Add Goal</button>
      </form>
    `);
  },

  async submitGoal(e) {
    e.preventDefault();
    const dateVal = document.getElementById('newGoalDate').value;
    try {
      await this.api('/council/goals', {
        method: 'POST',
        body: {
          id: 'goal-' + Date.now(),
          name: document.getElementById('newGoalName').value,
          description: document.getElementById('newGoalDesc').value,
          category: document.getElementById('newGoalCat').value,
          priority: parseInt(document.getElementById('newGoalPriority').value),
          target_amount: parseFloat(document.getElementById('newGoalTarget').value),
          current_amount: parseFloat(document.getElementById('newGoalCurrent').value) || 0,
          target_date: dateVal ? new Date(dateVal).toISOString() : undefined,
        },
      });
      this.closeModal();
      this.loadCouncil();
      this.flash('Goal added', 'info');
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  contributeGoal(goalId, goalName) {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Fund: ${goalName}</h3>
      <form onsubmit="App.submitContribution(event, '${goalId}')" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Amount ($)</label>
          <input type="number" id="contribAmount" class="input" placeholder="500" required>
        </div>
        <div class="form-row">
          <label>Source</label>
          <select id="contribSource" class="input">
            <option value="payout">Trading Payout</option>
            <option value="dividends">Dividends / Interest</option>
            <option value="manual">Manual / Other Income</option>
            <option value="allocation">Budget Allocation</option>
          </select>
        </div>
        <div class="form-row">
          <label>Note (optional)</label>
          <input type="text" id="contribNote" class="input" placeholder="From March payout">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Add Funds</button>
      </form>
    `);
  },

  async submitContribution(e, goalId) {
    e.preventDefault();
    try {
      await this.api(`/council/goals/${goalId}/contribute`, {
        method: 'POST',
        body: {
          id: 'contrib-' + Date.now(),
          amount: parseFloat(document.getElementById('contribAmount').value),
          source: document.getElementById('contribSource').value,
          note: document.getElementById('contribNote').value,
        },
      });
      this.closeModal();
      this.loadCouncil();
      this.flash('Contribution recorded', 'info');
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  addExpense() {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Add Expense / Bill</h3>
      <form onsubmit="App.submitExpense(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newExpName" class="input" placeholder="Rent, Electric, Data Feed..." required>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Category</label>
            <select id="newExpCat" class="input">
              <option value="rent">Rent/Mortgage</option>
              <option value="utilities">Utilities</option>
              <option value="subscriptions">Subscriptions</option>
              <option value="insurance">Insurance</option>
              <option value="trading_fees">Trading Fees</option>
              <option value="data_feeds">Data Feeds</option>
              <option value="software">Software</option>
              <option value="food">Food</option>
              <option value="transport">Transport</option>
              <option value="other">Other</option>
            </select>
          </div>
          <div>
            <label>Frequency</label>
            <select id="newExpFreq" class="input">
              <option value="monthly">Monthly</option>
              <option value="weekly">Weekly</option>
              <option value="biweekly">Biweekly</option>
              <option value="annual">Annual</option>
              <option value="one_time">One Time</option>
            </select>
          </div>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Amount ($)</label>
            <input type="number" id="newExpAmount" class="input" placeholder="1200" required>
          </div>
          <div>
            <label>Due Day (1-31)</label>
            <input type="number" id="newExpDue" class="input" value="1" min="1" max="31">
          </div>
        </div>
        <div class="form-row">
          <label style="display:flex;align-items:center;gap:8px">
            <input type="checkbox" id="newExpAuto"> Auto-Pay
          </label>
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Add Expense</button>
      </form>
    `);
  },

  async submitExpense(e) {
    e.preventDefault();
    try {
      await this.api('/council/expenses', {
        method: 'POST',
        body: {
          id: 'exp-' + Date.now(),
          name: document.getElementById('newExpName').value,
          category: document.getElementById('newExpCat').value,
          frequency: document.getElementById('newExpFreq').value,
          amount: parseFloat(document.getElementById('newExpAmount').value),
          due_day: parseInt(document.getElementById('newExpDue').value) || 1,
          auto_pay: document.getElementById('newExpAuto').checked,
        },
      });
      this.closeModal();
      this.loadCouncil();
      this.flash('Expense added', 'info');
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  payExpense(expenseId, expenseName, amount) {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Pay: ${expenseName}</h3>
      <form onsubmit="App.submitPayment(event, '${expenseId}')" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Amount ($)</label>
          <input type="number" id="paymentAmount" class="input" value="${amount}" required>
        </div>
        <div class="form-row">
          <label>Method</label>
          <select id="paymentMethod" class="input">
            <option value="trading_income">Trading Income</option>
            <option value="dividends">Dividends / Interest</option>
            <option value="bank">Bank Account</option>
            <option value="manual">Manual / Cash</option>
          </select>
        </div>
        <div class="form-row">
          <label>Note (optional)</label>
          <input type="text" id="paymentNote" class="input" placeholder="March payment">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Record Payment</button>
      </form>
    `);
  },

  async submitPayment(e, expenseId) {
    e.preventDefault();
    try {
      await this.api(`/council/expenses/${expenseId}/pay`, {
        method: 'POST',
        body: {
          id: 'pay-' + Date.now(),
          amount: parseFloat(document.getElementById('paymentAmount').value),
          method: document.getElementById('paymentMethod').value,
          note: document.getElementById('paymentNote').value,
        },
      });
      this.closeModal();
      this.loadCouncil();
      this.flash('Payment recorded', 'info');
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  async addAccount() {
    // Load prop firm registry so the form can offer the right defaults.
    const firms = (this.state.propFirms || []).length
      ? this.state.propFirms
      : await this.loadPropFirms();
    const firmOpts = firms.map(f => `<option value="${f.id}" data-split="${Math.round(f.profit_split*100)}">${esc(f.name)}</option>`).join('');
    this.openModal(`
      <h3 style="margin-bottom: 16px">Add Trading Account</h3>
      <form onsubmit="App.submitAccount(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>ID</label>
          <input type="text" id="newAcctId" class="input" placeholder="apex-1" required>
        </div>
        <div class="form-row">
          <label>Name</label>
          <input type="text" id="newAcctName" class="input" placeholder="Apex Account #1" required>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Broker</label>
            <select id="newAcctBroker" class="input">
              <option value="ibkr">IBKR</option>
              <option value="schwab">Schwab</option>
              <option value="tastytrade">TastyTrade</option>
              <option value="fidelity">Fidelity</option>
              <option value="tradovate">Tradovate</option>
              <option value="rithmic">Rithmic</option>
              <option value="topstepx">TopstepX</option>
              <option value="projectx">ProjectX</option>
              <option value="webull">Webull</option>
              <option value="other">Other</option>
            </select>
          </div>
          <div>
            <label>Type</label>
            <select id="newAcctType" class="input" onchange="App.onAcctTypeChange(this.value)">
              <option value="personal">Personal</option>
              <option value="retirement">Retirement (IRA/401k)</option>
              <option value="prop">Prop (Funded)</option>
              <option value="margin">Margin</option>
              <option value="paper">Paper</option>
            </select>
          </div>
        </div>
        <div class="form-row" id="newAcctPropFirmRow" style="display:none">
          <label>Prop Firm</label>
          <select id="newAcctPropFirm" class="input" onchange="App.onPropFirmChange()">
            <option value="">— select firm —</option>
            ${firmOpts}
          </select>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Initial Balance</label>
            <input type="number" id="newAcctBalance" class="input" value="50000">
          </div>
          <div>
            <label>Profit Split (%)</label>
            <input type="number" id="newAcctSplit" class="input" value="90" min="0" max="100">
          </div>
        </div>
        <div class="form-row">
          <label>Instruments</label>
          <input type="text" id="newAcctInstruments" class="input" placeholder="AAPL, ES, SPY, NQ (comma separated)">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Add Account</button>
      </form>
    `);
  },

  onAcctTypeChange(val) {
    const row = document.getElementById('newAcctPropFirmRow');
    if (row) row.style.display = val === 'prop' ? '' : 'none';
  },

  onPropFirmChange() {
    const sel = document.getElementById('newAcctPropFirm');
    const opt = sel.options[sel.selectedIndex];
    const split = opt && opt.dataset.split;
    if (split) document.getElementById('newAcctSplit').value = split;
  },

  async loadPropFirms() {
    try {
      const data = await this.api('/council/prop-firms');
      this.state.propFirms = data.firms || [];
      this.state.taxWithholdPct = data.tax_withhold_pct || 0.30;
      this.state.fireflyAccounts = data.firefly_asset_accounts || [];
      return this.state.propFirms;
    } catch (e) {
      this.state.propFirms = [];
      return [];
    }
  },

  async submitAccount(e) {
    e.preventDefault();
    const balance = parseFloat(document.getElementById('newAcctBalance').value);
    const type = document.getElementById('newAcctType').value;
    const propFirmEl = document.getElementById('newAcctPropFirm');
    try {
      await this.api('/council/accounts', {
        method: 'POST',
        body: {
          id: document.getElementById('newAcctId').value,
          name: document.getElementById('newAcctName').value,
          broker: document.getElementById('newAcctBroker').value,
          prop_firm: type === 'prop' && propFirmEl ? propFirmEl.value : '',
          type: type,
          initial_balance: balance,
          current_balance: balance,
          profit_split: parseFloat(document.getElementById('newAcctSplit').value) / 100,
          instruments: document.getElementById('newAcctInstruments').value.split(',').map(s => s.trim()).filter(Boolean),
        },
      });
      this.closeModal();
      this.loadCouncil();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  async recordPayout() {
    if (!this.state.propFirms) await this.loadPropFirms();
    const accounts = this.state.accounts.filter(a => a.status === 'active');
    const taxPct = this.state.taxWithholdPct || 0.30;
    const ffAccts = this.state.fireflyAccounts || [];
    const destOpts = ffAccts.length
      ? ffAccts.map(n => `<option value="${esc(n)}">${esc(n)}</option>`).join('')
      : `<option value="Personal Checking">Personal Checking</option>
         <option value="Savings">Savings</option>
         <option value="Bills">Bills</option>`;
    this.openModal(`
      <h3 style="margin-bottom: 16px">Record Payout</h3>
      <form onsubmit="App.submitPayout(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Account</label>
          <select id="payoutAcct" class="input" required onchange="App.updatePayoutEstimate()">
            ${accounts.map(a => `<option value="${a.id}" data-type="${a.type}" data-split="${a.profit_split||0.9}">${esc(a.name)} ($${fmt(a.current_balance)})</option>`).join('')}
            ${accounts.length === 0 ? '<option value="">No active accounts</option>' : ''}
          </select>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Gross Amount (before split)</label>
            <input type="number" id="payoutGross" class="input" placeholder="1000" required oninput="App.updatePayoutEstimate()">
          </div>
          <div>
            <label>Net Override (optional)</label>
            <input type="number" id="payoutNet" class="input" placeholder="auto = gross × split" oninput="App.updatePayoutEstimate()">
          </div>
        </div>
        <div id="payoutEstimate" style="padding:10px;border:1px solid var(--border);border-radius:6px;background:var(--bg-2);font-size:12px;line-height:1.6">
          Fill in gross amount to see split &amp; tax reserve.
        </div>
        <div class="form-row">
          <label>Destination (Firefly asset account)</label>
          <select id="payoutDest" class="input">
            ${destOpts}
          </select>
        </div>
        <div class="form-row">
          <label>Note (optional)</label>
          <input type="text" id="payoutNote" class="input" placeholder="Monthly withdrawal">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Record Payout</button>
      </form>
    `);
    this.updatePayoutEstimate();
  },

  updatePayoutEstimate() {
    const acctSel = document.getElementById('payoutAcct');
    const grossEl = document.getElementById('payoutGross');
    const netOverrideEl = document.getElementById('payoutNet');
    const estEl = document.getElementById('payoutEstimate');
    if (!acctSel || !grossEl || !estEl) return;
    const opt = acctSel.options[acctSel.selectedIndex];
    const isProp = opt && opt.dataset.type === 'prop';
    const split = parseFloat(opt && opt.dataset.split) || 0.9;
    const gross = parseFloat(grossEl.value) || 0;
    const netOverride = parseFloat(netOverrideEl && netOverrideEl.value);
    const net = Number.isFinite(netOverride) && netOverride > 0 ? netOverride : gross * split;
    const taxPct = this.state.taxWithholdPct || 0.30;
    const taxReserve = isProp ? net * taxPct : 0;
    const takeHome = net - taxReserve;
    estEl.innerHTML = `
      <div style="display:flex;justify-content:space-between"><span>Gross</span><span>$${fmt(gross)}</span></div>
      <div style="display:flex;justify-content:space-between"><span>Profit split (${Math.round(split*100)}%)</span><span>$${fmt(net)}</span></div>
      ${isProp ? `<div style="display:flex;justify-content:space-between;color:var(--red)"><span>Tax reserve (${Math.round(taxPct*100)}%)</span><span>−$${fmt(taxReserve)}</span></div>` : ''}
      <div style="display:flex;justify-content:space-between;font-weight:600;border-top:1px solid var(--border);margin-top:6px;padding-top:6px"><span>Take-home</span><span>$${fmt(takeHome)}</span></div>
      ${isProp ? '<div style="color:var(--text-3);font-size:11px;margin-top:4px">Tax reserve is a ledger allocation, not a transfer. Adjust via TRADING_TAX_WITHHOLD_PCT.</div>' : ''}
    `;
  },

  async submitPayout(e) {
    e.preventDefault();
    const acctSel = document.getElementById('payoutAcct');
    const opt = acctSel.options[acctSel.selectedIndex];
    const split = parseFloat(opt && opt.dataset.split) || 0.9;
    const gross = parseFloat(document.getElementById('payoutGross').value);
    const netOverride = parseFloat(document.getElementById('payoutNet').value);
    const net = Number.isFinite(netOverride) && netOverride > 0 ? netOverride : gross * split;
    try {
      await this.api('/council/payouts', {
        method: 'POST',
        body: {
          id: 'payout-' + Date.now(),
          account_id: acctSel.value,
          gross_amount: gross,
          net_amount: net,
          destination: document.getElementById('payoutDest').value,
          note: document.getElementById('payoutNote').value,
        },
      });
      this.closeModal();
      this.loadCouncil();
      this.flash('Payout recorded', 'info');
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  // ─── Prop Fee ─────────────────────────────────────────

  async recordPropFee(accountId, firmId) {
    if (!this.state.propFirms) await this.loadPropFirms();
    const firms = this.state.propFirms || [];
    const firmOpts = firms.map(f => `<option value="${f.id}" ${f.id===firmId?'selected':''} data-eval="${f.default_eval_fee}" data-act="${f.default_activation}" data-reset="${f.default_reset_fee}">${esc(f.name)}</option>`).join('');
    const ffAccts = this.state.fireflyAccounts || [];
    const sourceOpts = ffAccts.length
      ? ffAccts.map(n => `<option value="${esc(n)}">${esc(n)}</option>`).join('')
      : '<option value="Personal Checking">Personal Checking</option>';
    this.openModal(`
      <h3 style="margin-bottom:16px">Record Prop Fee</h3>
      <form onsubmit="App.submitPropFee(event, '${accountId||''}')" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row two-col">
          <div>
            <label>Prop Firm</label>
            <select id="feeFirm" class="input" required onchange="App.onFeeFirmOrTypeChange()">
              ${firmOpts}
            </select>
          </div>
          <div>
            <label>Fee Type</label>
            <select id="feeType" class="input" required onchange="App.onFeeFirmOrTypeChange()">
              <option value="eval">Evaluation / Combine purchase</option>
              <option value="activation">Funded activation</option>
              <option value="reset">Reset (after breach)</option>
              <option value="data">Market data</option>
              <option value="other">Other</option>
            </select>
          </div>
        </div>
        <div class="form-row two-col">
          <div>
            <label>Amount ($)</label>
            <input type="number" id="feeAmount" class="input" step="0.01" required>
          </div>
          <div>
            <label>Paid Date</label>
            <input type="date" id="feeDate" class="input" value="${new Date().toISOString().slice(0,10)}" required>
          </div>
        </div>
        <div class="form-row">
          <label>Source (Firefly asset account)</label>
          <select id="feeSource" class="input">${sourceOpts}</select>
        </div>
        <div class="form-row">
          <label>Note</label>
          <input type="text" id="feeNote" class="input" placeholder="e.g. Black Friday promo">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Record Fee</button>
      </form>
    `);
    this.onFeeFirmOrTypeChange();
  },

  onFeeFirmOrTypeChange() {
    const firmSel = document.getElementById('feeFirm');
    const typeSel = document.getElementById('feeType');
    const amtEl = document.getElementById('feeAmount');
    if (!firmSel || !typeSel || !amtEl) return;
    const opt = firmSel.options[firmSel.selectedIndex];
    if (!opt) return;
    const defaults = {eval: opt.dataset.eval, activation: opt.dataset.act, reset: opt.dataset.reset};
    const v = defaults[typeSel.value];
    if (v !== undefined && v !== '') amtEl.value = v;
  },

  async submitPropFee(e, accountId) {
    e.preventDefault();
    try {
      await this.api('/council/prop-fees', {
        method: 'POST',
        body: {
          account_id: accountId || '',
          prop_firm: document.getElementById('feeFirm').value,
          fee_type: document.getElementById('feeType').value,
          amount: parseFloat(document.getElementById('feeAmount').value),
          paid_date: document.getElementById('feeDate').value,
          source: document.getElementById('feeSource').value,
          note: document.getElementById('feeNote').value,
        },
      });
      this.closeModal();
      this.flash('Prop fee recorded', 'info');
      this.loadCouncil();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  // ─── Combine Rubicon ──────────────────────────────────

  async markCombinePassed(accountId) {
    if (!confirm('Mark this combine as PASSED? Stamps combine_pass_date only — funded activation is a separate step.')) return;
    try {
      const a = await this.api(`/council/accounts/${accountId}/mark-passed`, {method: 'POST'});
      this.flash(`Combine passed — ${a.name || accountId}. Activate funded account when ready.`, 'success');
      this.loadCouncil();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  async markAccountFunded(accountId) {
    if (!confirm('Mark this account as FUNDED? This is the Rubicon — moves to fxt phase (real money).')) return;
    try {
      const a = await this.api(`/council/accounts/${accountId}/mark-funded`, {method: 'POST'});
      this.flash(`🩸 Rubicon crossed — ${a.name || accountId} FUNDED`, 'success');
      this.loadCouncil();
    } catch (err) {
      this.flash(err.message, 'error');
    }
  },

  // ─── Render: Prop Desk ────────────────────────────────

  renderPropDesk() {
    const propAccounts = this.state.accounts.filter(a => a.type === 'prop');
    const propPayouts = this.state.payouts.filter(p =>
      propAccounts.some(a => a.id === p.account_id)
    );
    const propAdvice = this.state.withdrawalAdvice.filter(w =>
      propAccounts.some(a => a.id === w.account_id || a.name === w.account_name)
    );

    // Sort: phase class (live > fxt > combine > blown), then by return % descending
    const phaseOrder = { live: 0, fxt: 1, combine: 2, blown: 3 };
    const sorted = [...propAccounts].sort((a, b) => {
      const pa = a.account_phase || (a.status === 'blown' ? 'blown' : 'combine');
      const pb = b.account_phase || (b.status === 'blown' ? 'blown' : 'combine');
      const classDiff = (phaseOrder[pa] ?? 2) - (phaseOrder[pb] ?? 2);
      if (classDiff !== 0) return classDiff;
      // Within same phase: sort by return % descending
      const retA = a.initial_balance > 0 ? (a.current_balance - a.initial_balance) / a.initial_balance : 0;
      const retB = b.initial_balance > 0 ? (b.current_balance - b.initial_balance) / b.initial_balance : 0;
      return retB - retA;
    });

    this.renderPropBanner(propAccounts, propPayouts);
    this.renderPropAccountCards(sorted);
    this.renderPropWithdrawals(propAdvice);
    this.renderPropPayoutTable(propAccounts, propPayouts);
    this.renderPropMetrics(propAccounts, propPayouts);
  },

  renderPropBanner(accounts, payouts) {
    const el = document.getElementById('propBanner');
    const phase = a => a.account_phase || (a.status === 'blown' ? 'blown' : 'combine');

    const funded = accounts.filter(a => ['fxt', 'live'].includes(phase(a)) && a.status === 'active');
    const combines = accounts.filter(a => phase(a) === 'combine' && a.status === 'active');
    const blown = accounts.filter(a => phase(a) === 'blown' || a.status === 'blown');

    const sumBal = arr => arr.reduce((s, a) => s + a.current_balance, 0);
    const sumPnl = arr => arr.reduce((s, a) => s + (a.current_balance - a.initial_balance), 0);
    const pnlStr = v => `<span style="color:${v >= 0 ? 'var(--green)' : 'var(--red)'}">${v >= 0 ? '+$' : '-$'}${fmt(Math.abs(v))}</span>`;

    const fundedCapital = sumBal(funded);
    const fundedPnl = sumPnl(funded);
    const fundedAvail = funded.reduce((s, a) => s + Math.max(0, a.current_balance - a.initial_balance), 0);
    const fundedPayouts = funded.reduce((s, a) => s + a.total_payouts, 0);
    const avgSplit = funded.length > 0
      ? funded.reduce((s, a) => s + a.profit_split, 0) / funded.length : 0;

    const combCapital = sumBal(combines);
    const combPnl = sumPnl(combines);
    const combAvgProgress = combines.length > 0
      ? combines.reduce((s, a) => s + (a.combine_progress_pct || 0), 0) / combines.length : 0;

    const blownLost = blown.reduce((s, a) => s + Math.abs(a.current_balance - a.initial_balance), 0);

    const segments = [];

    // Funded segment — real cash value
    segments.push(`
      <div class="prop-banner-segment funded">
        <div class="prop-segment-header">
          <span class="prop-segment-title" style="color:var(--green)">Funded</span>
          <span class="prop-segment-count">${funded.length} account${funded.length !== 1 ? 's' : ''}</span>
        </div>
        <div class="prop-segment-stats">
          <div class="prop-segment-stat">
            <span class="stat-label">Capital</span>
            <span class="stat-value">$${fmt(fundedCapital)}</span>
          </div>
          <div class="prop-segment-stat">
            <span class="stat-label">P&L</span>
            <span class="stat-value">${pnlStr(fundedPnl)}</span>
          </div>
          <div class="prop-segment-stat">
            <span class="stat-label">Available</span>
            <span class="stat-value" style="color:var(--green)">$${fmt(fundedAvail)}</span>
          </div>
          <div class="prop-segment-stat">
            <span class="stat-label">Payouts</span>
            <span class="stat-value" style="color:var(--gold)">$${fmt(fundedPayouts)}</span>
          </div>
          <div class="prop-segment-stat">
            <span class="stat-label">Avg Split</span>
            <span class="stat-value">${Math.round(avgSplit * 100)}%</span>
          </div>
        </div>
      </div>`);

    // Evaluation segment — no cash value
    if (combines.length > 0) {
      segments.push(`
        <div class="prop-banner-segment evaluation">
          <div class="prop-segment-header">
            <span class="prop-segment-title" style="color:var(--accent)">Evaluation</span>
            <span class="prop-segment-count">${combines.length} combine${combines.length !== 1 ? 's' : ''}</span>
          </div>
          <div class="prop-segment-stats">
            <div class="prop-segment-stat">
              <span class="stat-label">Sim Capital</span>
              <span class="stat-value" style="color:var(--text-3)">$${fmt(combCapital)}</span>
            </div>
            <div class="prop-segment-stat">
              <span class="stat-label">Sim P&L</span>
              <span class="stat-value" style="color:var(--text-3)">${combPnl >= 0 ? '+' : '-'}$${fmt(Math.abs(combPnl))}</span>
            </div>
            <div class="prop-segment-stat">
              <span class="stat-label">Avg Progress</span>
              <span class="stat-value">${combAvgProgress.toFixed(0)}%</span>
            </div>
          </div>
        </div>`);
    }

    // Blown segment
    if (blown.length > 0) {
      segments.push(`
        <div class="prop-banner-segment inactive">
          <div class="prop-segment-header">
            <span class="prop-segment-title" style="color:var(--red)">Blown</span>
            <span class="prop-segment-count">${blown.length} account${blown.length !== 1 ? 's' : ''}</span>
          </div>
          <div class="prop-segment-stats">
            <div class="prop-segment-stat">
              <span class="stat-label">Fees Lost</span>
              <span class="stat-value" style="color:var(--red)">$${fmt(blownLost)}</span>
            </div>
          </div>
        </div>`);
    }

    el.innerHTML = `<div class="prop-banner-grid">${segments.join('')}</div>`;
  },

  renderPropAccountCards(accounts) {
    const el = document.getElementById('propAccountCards');
    if (!accounts.length) {
      el.innerHTML = '<div class="empty-state">No prop accounts — add funded accounts to track</div>';
      return;
    }

    el.innerHTML = accounts.map(a => {
      const profit = a.current_balance - a.initial_balance;
      const profitPct = a.initial_balance > 0 ? (profit / a.initial_balance * 100) : 0;
      const isActive = a.status === 'active';
      const phase = a.account_phase || (a.status === 'blown' ? 'blown' : 'combine');
      const phaseColors = { combine: 'var(--accent)', fxt: 'var(--green)', live: 'var(--gold)', blown: 'var(--red)' };
      const phaseLabels = { combine: 'COMBINE', fxt: 'FUNDED — RUBICON CROSSED', live: 'LIVE', blown: 'BLOWN' };
      const statusColor = phaseColors[phase] || 'var(--text-3)';
      const mllPct = a.mll_usage_pct || 0;
      const mllBarColor = mllPct > 80 ? 'var(--red)' : mllPct > 50 ? 'var(--gold)' : 'var(--green)';
      const combProg = a.combine_progress_pct || 0;
      const consistencyOk = (a.consistency_pct || 0) < 50;

      const firm = a.prop_firm ? (this.state.propFirms || []).find(f => f.id === a.prop_firm) : null;
      const firmLabel = firm ? firm.name : '';
      const canMarkPassed = phase === 'combine' && isActive && combProg >= 80 && !a.combine_pass_date;
      const canMarkFunded = !!a.combine_pass_date && !a.funded_date;
      return `
        <div class="prop-account-card ${a.status}">
          <div class="prop-acct-header">
            <div>
              <span class="prop-acct-name">${esc(a.name)}</span>
              <span class="prop-acct-broker">${esc(a.broker)}</span>
              ${firmLabel ? `<span style="color:var(--accent);font-size:11px;font-weight:600;margin-left:6px">${esc(firmLabel)}</span>` : ''}
              ${a.combine_number ? `<span style="color:var(--text-3);font-size:11px">#${a.combine_number}</span>` : ''}
            </div>
            <span class="prop-acct-status" style="color:${statusColor}">${phaseLabels[phase] || a.status.toUpperCase()}</span>
          </div>

          <div class="prop-acct-balances">
            <div class="prop-acct-bal">
              <span class="prop-bal-label">Balance</span>
              <span class="prop-bal-value">$${fmt(a.current_balance)}</span>
            </div>
            <div class="prop-acct-bal">
              <span class="prop-bal-label">Daily P&L</span>
              <span class="prop-bal-value ${(a.daily_pnl||0) >= 0 ? 'positive' : 'negative'}">${(a.daily_pnl||0) >= 0 ? '+' : ''}$${fmt(a.daily_pnl||0)}</span>
            </div>
            <div class="prop-acct-bal">
              <span class="prop-bal-label">Total P&L</span>
              <span class="prop-bal-value ${profit >= 0 ? 'positive' : 'negative'}">${profit >= 0 ? '+' : ''}$${fmt(profit)}</span>
            </div>
            <div class="prop-acct-bal">
              <span class="prop-bal-label">Return</span>
              <span class="prop-bal-value ${profitPct >= 0 ? 'positive' : 'negative'}">${profitPct >= 0 ? '+' : ''}${profitPct.toFixed(1)}%</span>
            </div>
          </div>

          ${phase === 'combine' && a.profit_target > 0 ? `
          <div class="prop-progress-section">
            <div class="prop-detail-row">
              <span>Combine Progress</span>
              <span class="positive">${combProg.toFixed(1)}% of $${fmt(a.profit_target)}</span>
            </div>
            <div class="progress-bar"><div class="progress-fill" style="width:${Math.min(combProg,100)}%;background:var(--green)"></div></div>
          </div>` : ''}

          ${a.max_loss_limit > 0 ? `
          <div class="prop-progress-section">
            <div class="prop-detail-row">
              <span>MLL Usage</span>
              <span style="color:${mllBarColor}">${mllPct.toFixed(1)}% ($${fmt(a.mll_headroom||0)} headroom)</span>
            </div>
            <div class="progress-bar"><div class="progress-fill" style="width:${Math.min(mllPct,100)}%;background:${mllBarColor}"></div></div>
          </div>` : ''}

          <div class="prop-acct-details">
            ${a.combine_start_date ? `<div class="prop-detail-row"><span>Combine Started</span><span>${a.combine_start_date}</span></div>` : ''}
            ${a.combine_pass_date ? `<div class="prop-detail-row"><span>Combine Passed</span><span class="positive">${a.combine_pass_date}</span></div>` : ''}
            ${a.funded_date ? `<div class="prop-detail-row rubicon-row"><span>Crossed the Rubicon</span><span class="positive" style="color:var(--gold);font-weight:700">${a.funded_date}</span></div>` : ''}
            ${a.blown_date ? `<div class="prop-detail-row"><span>Blown Date</span><span class="negative">${a.blown_date}</span></div>` : ''}
            <div class="prop-detail-row">
              <span>Profit Split</span>
              <span style="color:var(--gold)">${Math.round(a.profit_split * 100)}% / ${Math.round((1 - a.profit_split) * 100)}%</span>
            </div>
            ${a.withdrawal_available > 0 ? `<div class="prop-detail-row"><span>Available for Withdrawal</span><span class="positive">$${fmt(a.withdrawal_available)}</span></div>` : ''}
            <div class="prop-detail-row">
              <span>Payouts</span>
              <span>${a.payout_count} ($${fmt(a.total_payouts)} net)</span>
            </div>
            <div class="prop-detail-row">
              <span>Trading Days</span>
              <span>${a.winning_days || 0}W / ${(a.total_trading_days||0) - (a.winning_days||0)}L (${a.total_trading_days || 0} total)</span>
            </div>
            ${a.best_day_pnl ? `<div class="prop-detail-row">
              <span>Best Day</span>
              <span class="positive">$${fmt(a.best_day_pnl)}</span>
            </div>` : ''}
            ${a.consistency_pct ? `<div class="prop-detail-row">
              <span>Consistency Rule</span>
              <span style="color:${consistencyOk ? 'var(--green)' : 'var(--red)'}">${a.consistency_pct.toFixed(1)}% ${consistencyOk ? '(OK)' : '(WARNING > 50%)'}</span>
            </div>` : ''}
            ${a.avg_daily_pnl ? `<div class="prop-detail-row"><span>Avg Daily P&L</span><span class="${a.avg_daily_pnl >= 0 ? 'positive' : 'negative'}">$${Number(a.avg_daily_pnl).toFixed(0)}</span></div>` : ''}
            ${a.overall_win_rate ? `<div class="prop-detail-row"><span>Win Rate</span><span>${a.overall_win_rate.toFixed(0)}%</span></div>` : ''}
            <div class="prop-detail-row">
              <span>Instruments</span>
              <span style="font-family:var(--font-mono);font-size:11px">${(a.instruments || []).join(', ') || '—'}</span>
            </div>
          </div>

          <div class="prop-acct-actions" style="display:flex;gap:6px;flex-wrap:wrap;margin-top:8px">
            ${(phase === 'fxt' || phase === 'live') && isActive && profit > 500 ? `
              <button class="btn btn-sm btn-primary" onclick="App.recordPayoutForAccount('${a.id}', '${esc(a.name)}', ${a.current_balance})">Request Payout</button>` : ''}
            ${canMarkPassed ? `
              <button class="btn btn-sm" style="background:var(--gold);color:#000" onclick="App.markCombinePassed('${a.id}')">Mark Combine Passed</button>` : ''}
            ${canMarkFunded ? `
              <button class="btn btn-sm" style="background:var(--gold);color:#000" onclick="App.markAccountFunded('${a.id}')">🩸 Mark Funded (Rubicon)</button>` : ''}
            <button class="btn btn-sm btn-ghost" onclick="App.recordPropFee('${a.id}', '${a.prop_firm||''}')">+ Record Fee</button>
          </div>
        </div>`;
    }).join('');
  },

  renderPropWithdrawals(advice) {
    const el = document.getElementById('propWithdrawals');
    if (!advice.length) {
      el.innerHTML = '<div class="empty-state">Add prop accounts to see withdrawal advice</div>';
      return;
    }
    el.innerHTML = advice.map(w => `
      <div class="withdrawal-card urgency-${w.urgency}">
        <div class="withdrawal-header">
          <span class="withdrawal-account">${esc(w.account_name)}</span>
          <span class="withdrawal-urgency ${w.urgency}">${w.urgency}</span>
        </div>
        <div class="withdrawal-reason">${esc(w.reason)}</div>
        ${w.recommended_amount > 0 ? `
          <div class="withdrawal-amount">Withdraw: $${fmt(w.recommended_amount)}</div>
          <div class="withdrawal-splits">
            ${(w.allocations || []).map(a => `
              <span class="withdrawal-split">${a.category}: $${fmt(a.amount)}</span>
            `).join('')}
          </div>
        ` : ''}
      </div>
    `).join('');
  },

  renderPropPayoutTable(accounts, payouts) {
    const tbody = document.getElementById('propPayoutBody');
    if (!payouts.length) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty-state">No payouts recorded</td></tr>';
      return;
    }

    const acctMap = {};
    accounts.forEach(a => { acctMap[a.id] = a; });

    const sorted = [...payouts].sort((a, b) =>
      new Date(b.requested_at) - new Date(a.requested_at)
    );

    tbody.innerHTML = sorted.map(p => {
      const acct = acctMap[p.account_id];
      const split = acct ? Math.round(acct.profit_split * 100) : '—';
      const statusClass = p.status === 'completed' ? 'positive' : '';
      return `
        <tr>
          <td>${formatDate(p.requested_at)}</td>
          <td>${esc(acct ? acct.name : p.account_id)}</td>
          <td>$${fmt(p.gross_amount)}</td>
          <td>${split}%</td>
          <td class="positive">$${fmt(p.net_amount)}</td>
          <td>${esc(p.destination)}</td>
          <td class="${statusClass}">${(p.status || 'completed').toUpperCase()}</td>
        </tr>`;
    }).join('');
  },

  renderPropMetrics(accounts, payouts) {
    const now = new Date();
    const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
    const yearStart = new Date(now.getFullYear(), 0, 1);

    const monthPayouts = payouts.filter(p => new Date(p.requested_at) >= monthStart);
    const yearPayouts = payouts.filter(p => new Date(p.requested_at) >= yearStart);

    const monthlyGross = monthPayouts.reduce((s, p) => s + p.gross_amount, 0);
    const monthlyNet = monthPayouts.reduce((s, p) => s + p.net_amount, 0);
    const firmFeesYtd = yearPayouts.reduce((s, p) => s + (p.gross_amount - p.net_amount), 0);

    const active = accounts.filter(a => a.status === 'active');
    const funded = active.filter(a => ['fxt', 'live'].includes(a.account_phase));
    const closed = accounts.filter(a => ['blown', 'closed', 'graduated'].includes(a.status));

    const best = funded.length > 0
      ? funded.reduce((best, a) => a.total_pnl > best.total_pnl ? a : best, funded[0])
      : null;

    const avgPayoutsPerMonth = payouts.length > 0
      ? (payouts.length / Math.max(1, monthsBetween(
          new Date(payouts[payouts.length - 1].requested_at), now
        ))).toFixed(1)
      : '0';

    setText('propMonthlyGross', '$' + fmt(monthlyGross));
    setText('propMonthlyNet', '$' + fmt(monthlyNet));
    setText('propFirmFees', '$' + fmt(firmFeesYtd));
    setText('propMonthPayouts', monthPayouts.length);
    setText('propActiveCount', active.length);
    setText('propClosedCount', closed.length);
    setText('propBestAcct', best ? esc(best.name) : '—');
    setText('propPayoutRate', avgPayoutsPerMonth + '/mo');
  },

  recordPayoutForAccount(accountId, accountName, balance) {
    this.openModal(`
      <h3 style="margin-bottom: 16px">Request Payout — ${accountName}</h3>
      <form onsubmit="App.submitPayout(event)" style="display:flex;flex-direction:column;gap:12px">
        <input type="hidden" id="payoutAcct" value="${accountId}">
        <div class="form-row">
          <label>Gross Amount (before split)</label>
          <input type="number" id="payoutGross" class="input" placeholder="1000" required>
        </div>
        <div class="form-row">
          <label>Destination</label>
          <select id="payoutDest" class="input">
            <option value="bank">Bank Account</option>
            <option value="personal_trading">Personal Trading Account</option>
            <option value="savings">Savings</option>
            <option value="bills">Bills</option>
          </select>
        </div>
        <div class="form-row">
          <label>Note (optional)</label>
          <input type="text" id="payoutNote" class="input" placeholder="Monthly withdrawal">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Record Payout</button>
      </form>
    `);
  },

  // ─── Navigation ───────────────────────────────────────

  bindNav() {
    document.querySelectorAll('.nav-btn').forEach(btn => {
      btn.addEventListener('click', () => this.switchView(btn.dataset.view));
    });
    document.getElementById('killSwitchBtn').addEventListener('click', () => this.killSwitch());
  },

  switchView(view) {
    this.state.view = view;
    document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
    document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
    document.getElementById(`view-${view}`).classList.add('active');
    document.querySelector(`[data-view="${view}"]`).classList.add('active');
  },

  // ─── Keyboard ─────────────────────────────────────────

  bindKeyboard() {
    document.addEventListener('keydown', (e) => {
      // Don't intercept when typing in inputs
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') {
        if (e.key === 'Escape') {
          e.target.blur();
          this.closeCommandPalette();
          this.closeModal();
        }
        return;
      }

      // Number keys switch views
      if (e.key >= '1' && e.key <= '7') {
        const views = ['throne', 'tactical', 'forge', 'performance', 'council', 'prop', 'arsenal'];
        this.switchView(views[parseInt(e.key) - 1]);
        return;
      }

      // Ctrl+P — command palette
      if ((e.ctrlKey || e.metaKey) && e.key === 'p') {
        e.preventDefault();
        this.toggleCommandPalette();
        return;
      }

      // Ctrl+K — kill switch
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        this.killSwitch();
        return;
      }

      // Escape — close modals
      if (e.key === 'Escape') {
        this.closeCommandPalette();
        this.closeModal();
      }

      // R — refresh data
      if (e.key === 'r') {
        this.loadData();
        this.flash('Refreshed', 'info');
      }
    });
  },

  // ─── Command Palette ──────────────────────────────────

  toggleCommandPalette() {
    const el = document.getElementById('commandPalette');
    if (el.classList.contains('hidden')) {
      el.classList.remove('hidden');
      const input = document.getElementById('commandInput');
      input.value = '';
      input.focus();
      this.renderCommands('');
      input.oninput = () => this.renderCommands(input.value);
    } else {
      this.closeCommandPalette();
    }
  },

  closeCommandPalette() {
    document.getElementById('commandPalette').classList.add('hidden');
  },

  renderCommands(query) {
    const commands = [
      { icon: '⚔', label: 'Command Throne', key: '1', action: () => this.switchView('throne') },
      { icon: '◎', label: 'Tactical Display', key: '2', action: () => this.switchView('tactical') },
      { icon: '⚒', label: 'Forge Console', key: '3', action: () => this.switchView('forge') },
      { icon: '📊', label: 'Performance', key: '4', action: () => this.switchView('performance') },
      { icon: '⚖', label: 'Council', key: '5', action: () => this.switchView('council') },
      { icon: '⊕', label: 'Prop Desk', key: '6', action: () => this.switchView('prop') },
      { icon: '⎔', label: 'Arsenal (Holdings + Wheel)', key: '7', action: () => this.switchView('arsenal') },
      { icon: '+', label: 'Create Fortress', key: '', action: () => this.createFortress() },
      { icon: '⊘', label: 'Kill Switch', key: 'Ctrl+K', action: () => this.killSwitch() },
      { icon: '↻', label: 'Refresh Data', key: 'R', action: () => this.loadData() },
    ];

    // Add marines as commandable items
    this.state.marines.forEach(m => {
      commands.push({ icon: '▲', label: `Wake ${m.name}`, key: '', action: () => this.wakeMarine(m.id) });
      commands.push({ icon: '■', label: `Disable ${m.name}`, key: '', action: () => this.disableMarine(m.id) });
    });

    const q = query.toLowerCase();
    const filtered = commands.filter(c => c.label.toLowerCase().includes(q));

    document.getElementById('commandResults').innerHTML = filtered.map(c => `
      <div class="command-item" onclick="App.runCommand(this)" data-idx="${commands.indexOf(c)}">
        <span class="command-item-icon">${c.icon}</span>
        <span class="command-item-label">${esc(c.label)}</span>
        <span class="command-item-key">${c.key}</span>
      </div>
    `).join('');

    // Store commands for execution
    this._commands = commands;
  },

  runCommand(el) {
    const idx = parseInt(el.dataset.idx);
    if (this._commands && this._commands[idx]) {
      this.closeCommandPalette();
      this._commands[idx].action();
    }
  },

  // ─── Modal ────────────────────────────────────────────

  openModal(html) {
    document.getElementById('modalContent').innerHTML = html;
    document.getElementById('modal').classList.remove('hidden');
    // Focus first input
    setTimeout(() => {
      const input = document.querySelector('#modalContent input');
      if (input) input.focus();
    }, 50);
  },

  closeModal() {
    document.getElementById('modal').classList.add('hidden');
  },

  // ─── Flash Messages ───────────────────────────────────

  _flashCount: 0,

  flash(msg, type = 'info') {
    const offset = this._flashCount * 50;
    this._flashCount++;
    const el = document.createElement('div');
    el.style.cssText = `
      position: fixed; bottom: ${20 + offset}px; right: 20px; z-index: 3000;
      padding: 10px 18px; border-radius: 6px; font-size: 13px;
      animation: fadeIn 0.2s ease;
      background: ${type === 'error' ? 'var(--red-dim)' : 'var(--bg-3)'};
      border: 1px solid ${type === 'error' ? 'var(--red)' : 'var(--border-light)'};
      color: ${type === 'error' ? 'var(--red)' : 'var(--text-0)'};
      transition: opacity 0.3s, transform 0.3s;
    `;
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(() => {
      el.style.opacity = '0';
      el.style.transform = 'translateX(20px)';
      setTimeout(() => { el.remove(); this._flashCount = Math.max(0, this._flashCount - 1); }, 300);
    }, 3000);
  },
};

// ─── Login ────────────────────────────────────────────────

App.handleGoogleLogin = async function(response) {
  const errEl = document.getElementById('loginError');
  try {
    const r = await fetch('/api/v1/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ credential: response.credential }),
    });
    if (!r.ok) {
      const data = await r.json().catch(() => ({}));
      throw new Error(data.error || 'Authentication failed');
    }
    document.getElementById('loginOverlay').classList.add('hidden');
    App.startApp();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
  }
};

// ─── Holdings ─────────────────────────────────────────────

App.loadHoldings = async function() {
  try {
    App.state.holdings = await App.api('/holdings') || [];
    App.renderHoldings();
  } catch (e) {
    App.state.holdings = [];
  }
};

App.renderHoldings = function() {
  const tbody = document.getElementById('holdingsBody');
  if (!tbody) return;
  const h = App.state.holdings || [];
  if (h.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-state">No holdings — add your stock positions</td></tr>';
    return;
  }
  tbody.innerHTML = h.map(pos => `
    <tr>
      <td><strong>${esc(pos.symbol)}</strong></td>
      <td>${pos.quantity}</td>
      <td>$${Number(pos.avg_cost).toFixed(2)}</td>
      <td>$${(pos.quantity * pos.avg_cost).toFixed(2)}</td>
      <td>${Math.floor(pos.quantity / 100)}</td>
      <td>${esc(pos.notes || '')}</td>
      <td>
        <button class="btn btn-sm" onclick="App.editHolding('${pos.id}')">Edit</button>
        <button class="btn btn-sm" onclick="App.deleteHolding('${pos.id}')">Del</button>
      </td>
    </tr>
  `).join('');
};

App.addHolding = function() {
  App.openModal(`
    <h3 style="margin-bottom: 16px">Add Stock Holding</h3>
    <form onsubmit="App.submitHolding(event)" style="display:flex;flex-direction:column;gap:12px">
      <div class="form-row">
        <label>Symbol</label>
        <input type="text" id="holdSymbol" class="input" placeholder="RIVN" required style="text-transform:uppercase">
      </div>
      <div class="form-row two-col">
        <div>
          <label>Shares</label>
          <input type="number" id="holdQty" class="input" placeholder="100" step="1" required>
        </div>
        <div>
          <label>Avg Cost ($)</label>
          <input type="number" id="holdCost" class="input" placeholder="12.50" step="0.01" required>
        </div>
      </div>
      <div class="form-row">
        <label>Notes (optional)</label>
        <input type="text" id="holdNotes" class="input" placeholder="Wheeling since Jan 2026...">
      </div>
      <button type="submit" class="btn btn-primary btn-lg">Add Holding</button>
    </form>
  `);
};

App.submitHolding = async function(e) {
  e.preventDefault();
  try {
    await App.api('/holdings', { method: 'POST', body: {
      symbol: document.getElementById('holdSymbol').value.toUpperCase(),
      quantity: parseFloat(document.getElementById('holdQty').value),
      avg_cost: parseFloat(document.getElementById('holdCost').value),
      notes: document.getElementById('holdNotes').value,
    }});
    App.closeModal();
    App.flash('Holding added');
    App.loadHoldings();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

App.editHolding = function(id) {
  const h = (App.state.holdings || []).find(x => x.id === id);
  if (!h) return;
  App.openModal(`
    <h3 style="margin-bottom: 16px">Edit ${esc(h.symbol)} Holding</h3>
    <form onsubmit="App.submitEditHolding(event, '${id}')" style="display:flex;flex-direction:column;gap:12px">
      <div class="form-row two-col">
        <div>
          <label>Shares</label>
          <input type="number" id="editHoldQty" class="input" value="${h.quantity}" step="1" required>
        </div>
        <div>
          <label>Avg Cost ($)</label>
          <input type="number" id="editHoldCost" class="input" value="${h.avg_cost}" step="0.01" required>
        </div>
      </div>
      <div class="form-row">
        <label>Notes</label>
        <input type="text" id="editHoldNotes" class="input" value="${esc(h.notes || '')}">
      </div>
      <button type="submit" class="btn btn-primary btn-lg">Save</button>
    </form>
  `);
};

App.submitEditHolding = async function(e, id) {
  e.preventDefault();
  const h = (App.state.holdings || []).find(x => x.id === id);
  if (!h) return;
  try {
    await App.api(`/holdings/${id}`, { method: 'PUT', body: {
      symbol: h.symbol,
      quantity: parseFloat(document.getElementById('editHoldQty').value),
      avg_cost: parseFloat(document.getElementById('editHoldCost').value),
      notes: document.getElementById('editHoldNotes').value,
    }});
    App.closeModal();
    App.flash('Holding updated');
    App.loadHoldings();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

App.deleteHolding = async function(id) {
  if (!confirm('Delete this holding?')) return;
  try {
    await fetch(`/api/v1/holdings/${id}`, { method: 'DELETE' });
    App.flash('Holding deleted');
    App.loadHoldings();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

// ─── Wheel Analysis ──────────────────────────────────────

App.runWheelAnalysis = async function() {
  const el = document.getElementById('wheelAnalysis');
  el.innerHTML = '<div class="empty-state">Loading wheel analysis...</div>';
  try {
    const data = await App.api('/wheel-analysis');
    if (!data || data.length === 0) {
      el.innerHTML = '<div class="empty-state">No holdings to analyze. Add your stock positions first.</div>';
      return;
    }
    el.innerHTML = data.map(a => `
      <div class="wheel-symbol">
        <div class="wheel-header">
          <h3>${esc(a.symbol)}</h3>
          <span class="wheel-info">${a.quantity} shares @ $${Number(a.avg_cost).toFixed(2)} | ${Math.floor(a.quantity/100)} lots</span>
        </div>

        ${a.covered_calls && a.covered_calls.length > 0 ? `
          <div class="wheel-section">
            <h4>Covered Calls (sell to collect premium)</h4>
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr><th>Exp</th><th>Strike</th><th>Bid</th><th>Ask</th><th>Mark</th><th>DTE</th><th>$/Day</th><th>Ann %</th><th>Vol</th><th>OI</th></tr>
                </thead>
                <tbody>
                  ${a.covered_calls.map(o => `
                    <tr>
                      <td>${o.expiration}</td>
                      <td>$${o.strike.toFixed(2)}</td>
                      <td>$${o.bid.toFixed(2)}</td>
                      <td>$${o.ask.toFixed(2)}</td>
                      <td>$${o.mark.toFixed(2)}</td>
                      <td>${o.dte}</td>
                      <td class="positive">$${o.premium_per_day.toFixed(3)}</td>
                      <td class="positive">${o.annual_return_pct.toFixed(1)}%</td>
                      <td>${o.volume}</td>
                      <td>${o.open_interest}</td>
                    </tr>
                  `).join('')}
                </tbody>
              </table>
            </div>
          </div>
        ` : '<div class="wheel-section"><h4>Covered Calls</h4><div class="empty-state">Need 100+ shares for covered calls</div></div>'}

        ${a.cash_secured_puts && a.cash_secured_puts.length > 0 ? `
          <div class="wheel-section">
            <h4>Cash-Secured Puts (sell to buy lower / collect premium)</h4>
            <div class="table-wrap">
              <table class="data-table">
                <thead>
                  <tr><th>Exp</th><th>Strike</th><th>Bid</th><th>Ask</th><th>Mark</th><th>DTE</th><th>$/Day</th><th>Ann %</th><th>Vol</th><th>OI</th></tr>
                </thead>
                <tbody>
                  ${a.cash_secured_puts.map(o => `
                    <tr>
                      <td>${o.expiration}</td>
                      <td>$${o.strike.toFixed(2)}</td>
                      <td>$${o.bid.toFixed(2)}</td>
                      <td>$${o.ask.toFixed(2)}</td>
                      <td>$${o.mark.toFixed(2)}</td>
                      <td>${o.dte}</td>
                      <td class="positive">$${o.premium_per_day.toFixed(3)}</td>
                      <td class="positive">${o.annual_return_pct.toFixed(1)}%</td>
                      <td>${o.volume}</td>
                      <td>${o.open_interest}</td>
                    </tr>
                  `).join('')}
                </tbody>
              </table>
            </div>
          </div>
        ` : '<div class="wheel-section"><h4>Cash-Secured Puts</h4><div class="empty-state">No put data available</div></div>'}
      </div>
    `).join('');
  } catch (err) {
    el.innerHTML = `<div class="empty-state">Error: ${esc(err.message)}</div>`;
  }
};

// ─── Wheel Cycle Manager (Phalanx) ──────────────────────

App.loadWheelCycles = async function() {
  try {
    App.state.wheelCycles = await App.api('/wheel-cycles') || [];
    App.renderWheelCycles();
  } catch (e) {
    App.state.wheelCycles = [];
  }
};

App.renderWheelCycles = function() {
  const el = document.getElementById('wheelCycles');
  if (!el) return;
  const cycles = App.state.wheelCycles || [];
  if (cycles.length === 0) {
    el.innerHTML = '<div class="empty-state">No active wheels — start a new wheel cycle</div>';
    return;
  }

  const statusIcons = {
    'selling_puts': 'P',
    'assigned': 'A',
    'selling_calls': 'C',
    'called_away': 'X',
    'closed': '-'
  };
  const statusLabels = {
    'selling_puts': 'Selling Puts',
    'assigned': 'Assigned (Holding Stock)',
    'selling_calls': 'Selling Calls',
    'called_away': 'Called Away',
    'closed': 'Closed'
  };

  el.innerHTML = cycles.map(c => {
    const premium = Number(c.total_premium_collected || 0);
    const statusClass = c.status === 'closed' || c.status === 'called_away' ? 'neutral' : 'active';
    return `
    <div class="wheel-cycle-card ${statusClass}">
      <div class="wheel-cycle-header">
        <div>
          <strong>${esc(c.underlying)}</strong>
          <span class="badge badge-${c.status}">${statusLabels[c.status] || c.status}</span>
          <span class="badge badge-mode">${c.mode}</span>
        </div>
        <div>
          <span class="premium ${premium >= 0 ? 'positive' : 'negative'}">$${premium.toFixed(2)} premium</span>
          ${c.shares_held > 0 ? `<span class="shares">${c.shares_held} shares @ $${Number(c.cost_basis || 0).toFixed(2)}</span>` : ''}
        </div>
      </div>
      <div class="wheel-cycle-actions">
        <button class="btn btn-sm" onclick="App.addWheelLeg('${c.id}', '${esc(c.underlying)}')">+ Add Leg</button>
        <button class="btn btn-sm" onclick="App.viewWheelLegs('${c.id}', '${esc(c.underlying)}')">View Legs</button>
        <button class="btn btn-sm" onclick="App.updateWheelStatus('${c.id}')">Update Status</button>
      </div>
    </div>`;
  }).join('');
};

App.addWheelCycle = function() {
  App.openModal(`
    <h3 style="margin-bottom: 16px">Start New Wheel</h3>
    <form onsubmit="App.submitWheelCycle(event)" style="display:flex;flex-direction:column;gap:12px">
      <div class="form-row">
        <label>Underlying Symbol</label>
        <input type="text" id="wcSymbol" class="input" placeholder="RIVN" required style="text-transform:uppercase">
      </div>
      <div class="form-row">
        <label>Broker</label>
        <input type="text" id="wcBroker" class="input" placeholder="Fidelity, Schwab, etc.">
      </div>
      <div class="form-row">
        <label>Starting Phase</label>
        <select id="wcStatus" class="input">
          <option value="selling_puts">Selling Puts (CSP)</option>
          <option value="assigned">Assigned (Holding Stock)</option>
          <option value="selling_calls">Selling Calls (CC)</option>
        </select>
      </div>
      <button type="submit" class="btn btn-primary btn-lg">Start Wheel</button>
    </form>
  `);
};

App.submitWheelCycle = async function(e) {
  e.preventDefault();
  try {
    await App.api('/wheel-cycles', { method: 'POST', body: {
      underlying: document.getElementById('wcSymbol').value.toUpperCase(),
      broker: document.getElementById('wcBroker').value,
      status: document.getElementById('wcStatus').value,
      mode: 'manual',
    }});
    App.closeModal();
    App.flash('Wheel cycle started');
    App.loadWheelCycles();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

App.addWheelLeg = function(cycleId, underlying) {
  App.openModal(`
    <h3 style="margin-bottom: 16px">Add Leg — ${underlying}</h3>
    <form onsubmit="App.submitWheelLeg(event, '${cycleId}')" style="display:flex;flex-direction:column;gap:12px">
      <div class="form-row">
        <label>Leg Type</label>
        <select id="wlType" class="input">
          <option value="csp">Cash-Secured Put (CSP)</option>
          <option value="covered_call">Covered Call (CC)</option>
          <option value="assignment">Assignment (Got Shares)</option>
          <option value="called_away">Called Away (Lost Shares)</option>
          <option value="roll">Roll</option>
          <option value="close">Close Position</option>
        </select>
      </div>
      <div class="form-row two-col">
        <div>
          <label>Strike</label>
          <input type="number" id="wlStrike" class="input" step="0.5" placeholder="12.00">
        </div>
        <div>
          <label>Expiration</label>
          <input type="date" id="wlExpiration" class="input">
        </div>
      </div>
      <div class="form-row two-col">
        <div>
          <label>Option Type</label>
          <select id="wlOptType" class="input">
            <option value="P">Put</option>
            <option value="C">Call</option>
          </select>
        </div>
        <div>
          <label>Contracts</label>
          <input type="number" id="wlQty" class="input" value="1" step="1">
        </div>
      </div>
      <div class="form-row two-col">
        <div>
          <label>Premium (credit +, debit -)</label>
          <input type="number" id="wlPremium" class="input" step="0.01" placeholder="0.45">
        </div>
        <div>
          <label>Fill Price</label>
          <input type="number" id="wlFill" class="input" step="0.01" placeholder="0.45">
        </div>
      </div>
      <div class="form-row">
        <label>Notes</label>
        <input type="text" id="wlNotes" class="input" placeholder="Opened at 30 delta, 45 DTE">
      </div>
      <button type="submit" class="btn btn-primary btn-lg">Add Leg</button>
    </form>
  `);
};

App.submitWheelLeg = async function(e, cycleId) {
  e.preventDefault();
  try {
    await App.api(`/wheel-cycles/${cycleId}/legs`, { method: 'POST', body: {
      leg_type: document.getElementById('wlType').value,
      symbol: document.getElementById('wlStrike').value ? `${cycleId}` : '',
      strike: parseFloat(document.getElementById('wlStrike').value) || 0,
      expiration: document.getElementById('wlExpiration').value,
      option_type: document.getElementById('wlOptType').value,
      quantity: parseInt(document.getElementById('wlQty').value) || 1,
      premium: parseFloat(document.getElementById('wlPremium').value) || 0,
      fill_price: parseFloat(document.getElementById('wlFill').value) || 0,
      notes: document.getElementById('wlNotes').value,
    }});
    App.closeModal();
    App.flash('Leg added');
    App.loadWheelCycles();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

App.viewWheelLegs = async function(cycleId, underlying) {
  try {
    const legs = await App.api(`/wheel-cycles/${cycleId}/legs`) || [];
    if (legs.length === 0) {
      App.openModal(`<h3>${underlying} — Legs</h3><div class="empty-state">No legs recorded yet</div>`);
      return;
    }
    const html = legs.map(l => `
      <tr>
        <td>${esc(l.leg_type)}</td>
        <td>${l.option_type || '-'}</td>
        <td>${l.strike ? '$' + Number(l.strike).toFixed(2) : '-'}</td>
        <td>${l.expiration || '-'}</td>
        <td>${l.quantity || '-'}</td>
        <td class="${(l.premium||0) >= 0 ? 'positive' : 'negative'}">$${Number(l.premium||0).toFixed(2)}</td>
        <td>${esc(l.status)}</td>
        <td>${esc(l.notes || '')}</td>
      </tr>
    `).join('');
    App.openModal(`
      <h3>${underlying} — Legs</h3>
      <div class="table-wrap" style="margin-top:12px">
        <table class="data-table">
          <thead><tr><th>Type</th><th>P/C</th><th>Strike</th><th>Exp</th><th>Qty</th><th>Premium</th><th>Status</th><th>Notes</th></tr></thead>
          <tbody>${html}</tbody>
        </table>
      </div>
    `);
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

App.updateWheelStatus = function(cycleId) {
  const c = (App.state.wheelCycles || []).find(x => x.id === cycleId);
  if (!c) return;
  App.openModal(`
    <h3 style="margin-bottom: 16px">Update ${esc(c.underlying)} Wheel</h3>
    <form onsubmit="App.submitWheelUpdate(event, '${cycleId}')" style="display:flex;flex-direction:column;gap:12px">
      <div class="form-row">
        <label>Status</label>
        <select id="wuStatus" class="input">
          <option value="selling_puts" ${c.status==='selling_puts'?'selected':''}>Selling Puts</option>
          <option value="assigned" ${c.status==='assigned'?'selected':''}>Assigned (Holding Stock)</option>
          <option value="selling_calls" ${c.status==='selling_calls'?'selected':''}>Selling Calls</option>
          <option value="called_away" ${c.status==='called_away'?'selected':''}>Called Away</option>
          <option value="closed" ${c.status==='closed'?'selected':''}>Closed</option>
        </select>
      </div>
      <div class="form-row two-col">
        <div>
          <label>Cost Basis ($)</label>
          <input type="number" id="wuCost" class="input" value="${c.cost_basis||''}" step="0.01">
        </div>
        <div>
          <label>Shares Held</label>
          <input type="number" id="wuShares" class="input" value="${c.shares_held||0}" step="100">
        </div>
      </div>
      <button type="submit" class="btn btn-primary btn-lg">Update</button>
    </form>
  `);
};

App.submitWheelUpdate = async function(e, cycleId) {
  e.preventDefault();
  try {
    await App.api(`/wheel-cycles/${cycleId}`, { method: 'PUT', body: {
      status: document.getElementById('wuStatus').value,
      cost_basis: parseFloat(document.getElementById('wuCost').value) || 0,
      shares_held: parseInt(document.getElementById('wuShares').value) || 0,
    }});
    App.closeModal();
    App.flash('Wheel updated');
    App.loadWheelCycles();
  } catch (err) {
    App.flash('Error: ' + err.message, 'error');
  }
};

// ─── Helpers ──────────────────────────────────────────────

function esc(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function formatTime(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function formatDate(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function fmt(n) {
  if (n == null) return '0';
  return Number(n).toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 });
}

function monthsBetween(d1, d2) {
  return (d2.getFullYear() - d1.getFullYear()) * 12 + d2.getMonth() - d1.getMonth() + 1;
}

function setText(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val;
}

// ─── Boot ─────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => App.init());
