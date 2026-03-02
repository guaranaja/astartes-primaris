// ═══════════════════════════════════════════════════════════
// AURUM — Astartes Primaris Dashboard
// ═══════════════════════════════════════════════════════════

const App = {
  state: {
    view: 'throne',
    fortresses: [],
    marines: [],
    events: [],
    connected: false,
    // Council
    roadmap: null,
    accounts: [],
    payouts: [],
    budget: null,
    allocations: [],
    withdrawalAdvice: [],
    metrics: null,
  },

  // ─── Init ──────────────────────────────────────────────

  init() {
    this.bindNav();
    this.bindKeyboard();
    this.connectSSE();
    this.pollStatus();
    this.loadData();
    this.loadCouncil();
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
      const [fortresses, marines] = await Promise.all([
        this.api('/fortresses'),
        this.api('/marines'),
      ]);
      this.state.fortresses = fortresses || [];
      this.state.marines = marines || [];
      this.renderThrone();
      this.renderTactical();
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
      return `
        <div class="fortress-card" onclick="App.viewFortress('${f.id}')">
          <div class="fortress-header">
            <span class="fortress-name">${esc(f.name)}</span>
            <span class="fortress-class">${esc(f.asset_class)}</span>
          </div>
          ${companies.map(c => {
            const marines = c.marines || [];
            const activeCount = marines.filter(m => !['dormant','disabled','failed'].includes(m.status)).length;
            return `
              <div class="company-row">
                <span class="company-name">${esc(c.name)}</span>
                <span class="company-marines">
                  ${marines.length} marines
                  ${activeCount > 0 ? `<span class="marine-pill active">${activeCount} active</span>` : ''}
                </span>
              </div>`;
          }).join('')}
          ${companies.length === 0 ? '<div class="company-row"><span class="company-name" style="color:var(--text-3)">No companies</span></div>' : ''}
        </div>`;
    }).join('');
  },

  // ─── Render: Tactical ─────────────────────────────────

  renderTactical() {
    const container = document.getElementById('tacticalMarines');
    const marines = this.state.marines;

    if (!marines.length) {
      container.innerHTML = '<div class="empty-state">No marines registered. Add marines via the API or Command Throne.</div>';
      return;
    }

    container.innerHTML = marines.map(m => {
      const statusClass = ['waking','orienting','deciding','acting','reporting'].includes(m.status) ? 'active' : m.status;
      return `
        <div class="marine-card" data-marine-id="${m.id}">
          <div class="marine-status-dot ${statusClass}"></div>
          <div class="marine-info">
            <div class="marine-name">${esc(m.name)}</div>
            <div class="marine-detail">${esc(m.strategy_name)} @ ${esc(m.broker_account_id || 'unassigned')}</div>
          </div>
          <div style="font-family: var(--font-mono); font-size: 11px; color: var(--text-2)">
            ${m.status}
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

    container.innerHTML = this.state.events.slice(0, 50).map(e => `
      <div class="event-item">
        <span class="event-time">${formatTime(e.timestamp)}</span>
        <span class="event-icon">${icons[e.event] || '•'}</span>
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
            <option value="futures">Futures</option>
            <option value="options">Options</option>
            <option value="equities">Equities</option>
            <option value="crypto">Crypto</option>
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
      const [roadmap, accounts, payouts, budget, allocations, advice, metrics] = await Promise.all([
        this.api('/council/roadmap'),
        this.api('/council/accounts'),
        this.api('/council/payouts'),
        this.api('/council/budget'),
        this.api('/council/allocations'),
        this.api('/council/withdrawal-advice'),
        this.api('/council/metrics'),
      ]);
      this.state.roadmap = roadmap;
      this.state.accounts = accounts || [];
      this.state.payouts = payouts || [];
      this.state.budget = budget;
      this.state.allocations = allocations || [];
      this.state.withdrawalAdvice = advice || [];
      this.state.metrics = metrics;
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
    this.renderGoalTracker();
  },

  renderPhaseTracker() {
    const rm = this.state.roadmap;
    if (!rm) return;
    const el = document.getElementById('phaseTracker');
    const label = document.getElementById('currentPhaseLabel');
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
      return `
        <div class="phase-card ${cls}">
          ${cls === 'completed' ? '<span class="phase-check">✓</span>' : ''}
          <div class="phase-rank">${esc(p.title)}</div>
          <div class="phase-name">${esc(p.name)}</div>
          <div class="phase-desc">${esc(p.description).substring(0, 80)}${p.description.length > 80 ? '...' : ''}</div>
        </div>`;
    }).join('');
  },

  renderAccounts() {
    const el = document.getElementById('accountList');
    if (!this.state.accounts.length) {
      el.innerHTML = '<div class="empty-state">No accounts yet. Add your prop accounts to start tracking.</div>';
      return;
    }
    el.innerHTML = this.state.accounts.map(a => {
      const pnlClass = a.total_pnl >= 0 ? 'positive' : 'negative';
      return `
        <div class="account-card">
          <span class="account-type-badge ${a.type}">${a.type}</span>
          <div class="account-info">
            <div class="account-name">${esc(a.name)}</div>
            <div class="account-detail">${esc(a.broker)} · ${(a.instruments || []).join(', ') || 'N/A'} · ${a.payout_count} payouts</div>
          </div>
          <div class="account-balance">
            <div class="account-balance-value">$${fmt(a.current_balance)}</div>
            <div class="account-balance-pnl ${pnlClass}">${a.total_pnl >= 0 ? '+' : ''}$${fmt(a.total_pnl)} · ${Math.round(a.profit_split * 100)}% split</div>
          </div>
        </div>`;
    }).join('');
  },

  renderWithdrawalAdvice() {
    const el = document.getElementById('withdrawalAdvice');
    if (!this.state.withdrawalAdvice.length) {
      el.innerHTML = '<div class="empty-state">Add prop accounts to get withdrawal advice</div>';
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
    setText('bizLifetimePnl', '$' + fmt(m.lifetime_pnl));
    setText('bizLifetimePayouts', '$' + fmt(m.lifetime_payouts));
    setText('bizMonthlyNet', '$' + fmt(m.monthly_net_income));
    setText('bizPersonalValue', '$' + fmt(m.personal_account_value));
    setText('bizGoalProgress', Math.round(m.goal_progress * 100) + '%');
    setText('bizPhase', (m.current_phase || '—').replace('_', ' '));
    setText('bizProfitDays', m.profitable_days + '/' + m.total_trading_days);
    setText('bizBlown', m.accounts_blown);
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

  renderGoalTracker() {
    const m = this.state.metrics;
    if (!m) return;
    const pct = Math.min(m.goal_progress * 100, 100);
    setText('goalTitle', `Grow personal account to $${fmt(m.personal_account_goal)}`);
    setText('goalCurrent', '$' + fmt(m.personal_account_value));
    setText('goalTarget', '$' + fmt(m.personal_account_goal));
    document.getElementById('goalFill').style.width = pct + '%';
  },

  addAccount() {
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
              <option value="apex">Apex</option>
              <option value="projectx">ProjectX</option>
              <option value="ibkr">IBKR</option>
              <option value="tastytrade">TastyTrade</option>
            </select>
          </div>
          <div>
            <label>Type</label>
            <select id="newAcctType" class="input">
              <option value="prop">Prop (Funded)</option>
              <option value="personal">Personal</option>
              <option value="paper">Paper</option>
            </select>
          </div>
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
          <input type="text" id="newAcctInstruments" class="input" placeholder="MES, ES, NQ (comma separated)">
        </div>
        <button type="submit" class="btn btn-primary btn-lg">Add Account</button>
      </form>
    `);
  },

  async submitAccount(e) {
    e.preventDefault();
    const balance = parseFloat(document.getElementById('newAcctBalance').value);
    try {
      await this.api('/council/accounts', {
        method: 'POST',
        body: {
          id: document.getElementById('newAcctId').value,
          name: document.getElementById('newAcctName').value,
          broker: document.getElementById('newAcctBroker').value,
          type: document.getElementById('newAcctType').value,
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

  recordPayout() {
    const accounts = this.state.accounts.filter(a => a.type === 'prop' && a.status === 'active');
    this.openModal(`
      <h3 style="margin-bottom: 16px">Record Payout</h3>
      <form onsubmit="App.submitPayout(event)" style="display:flex;flex-direction:column;gap:12px">
        <div class="form-row">
          <label>Account</label>
          <select id="payoutAcct" class="input" required>
            ${accounts.map(a => `<option value="${a.id}">${esc(a.name)} ($${fmt(a.current_balance)})</option>`).join('')}
            ${accounts.length === 0 ? '<option value="">No prop accounts</option>' : ''}
          </select>
        </div>
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

  async submitPayout(e) {
    e.preventDefault();
    try {
      await this.api('/council/payouts', {
        method: 'POST',
        body: {
          id: 'payout-' + Date.now(),
          account_id: document.getElementById('payoutAcct').value,
          gross_amount: parseFloat(document.getElementById('payoutGross').value),
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
      if (e.key >= '1' && e.key <= '5') {
        const views = ['throne', 'tactical', 'forge', 'performance', 'council'];
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

  flash(msg, type = 'info') {
    const el = document.createElement('div');
    el.style.cssText = `
      position: fixed; bottom: 20px; right: 20px; z-index: 3000;
      padding: 10px 18px; border-radius: 6px; font-size: 13px;
      animation: fadeIn 0.2s ease;
      background: ${type === 'error' ? 'var(--red-dim)' : 'var(--bg-3)'};
      border: 1px solid ${type === 'error' ? 'var(--red)' : 'var(--border-light)'};
      color: ${type === 'error' ? 'var(--red)' : 'var(--text-0)'};
    `;
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(() => el.remove(), 3000);
  },
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

function setText(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val;
}

// ─── Boot ─────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => App.init());
