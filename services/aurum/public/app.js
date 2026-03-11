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
  },

  // ─── Init ──────────────────────────────────────────────

  async init() {
    // Check auth first
    try {
      await this.api('/auth-check');
    } catch (e) {
      document.getElementById('loginOverlay').classList.remove('hidden');
      return;
    }
    this.bindNav();
    this.bindKeyboard();
    this.connectSSE();
    this.pollStatus();
    this.loadData();
    this.loadCouncil();
    this.loadHoldings();
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

    // Summary banner
    setText('sumFortresses', status.fortresses || 0);
    setText('sumMarines', status.marines?.total || 0);
    setText('sumActive', status.marines?.active || 0);
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
      const [roadmap, accounts, payouts, budget, allocations, advice, metrics, goals, expenses, billing] = await Promise.all([
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
      ]);
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
    this.renderSummaryBanner();
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

  renderGoals() {
    const el = document.getElementById('goalList');
    if (!this.state.goals.length) {
      el.innerHTML = '<div class="empty-state">No goals yet — add your first target</div>';
      return;
    }
    // Sort by priority (1 = highest), then by completion status
    const sorted = [...this.state.goals].sort((a, b) => {
      if (a.status === 'completed' && b.status !== 'completed') return 1;
      if (b.status === 'completed' && a.status !== 'completed') return -1;
      return (a.priority || 3) - (b.priority || 3);
    });
    const categoryIcons = {
      home_improvement: '🏠', vehicle: '🚗', savings: '💰',
      trading: '📈', debt: '💳', lifestyle: '🎯', business: '🏢',
    };
    el.innerHTML = sorted.map(g => {
      const pct = g.target_amount > 0 ? Math.min((g.current_amount / g.target_amount) * 100, 100) : 0;
      const icon = g.icon || categoryIcons[g.category] || '🎯';
      const priorityDots = Array.from({length: 5}, (_, i) =>
        `<span class="goal-priority-dot ${i < (6 - (g.priority || 3)) ? 'filled' : ''}"></span>`
      ).join('');
      return `
        <div class="goal-card ${g.status}">
          <div class="goal-header">
            <span class="goal-name"><span class="goal-icon">${icon}</span> ${esc(g.name)}</span>
            <span class="goal-category ${g.category}">${(g.category || '').replace('_', ' ')}</span>
          </div>
          ${g.description ? `<div style="font-size:11px;color:var(--text-2);margin-bottom:6px">${esc(g.description)}</div>` : ''}
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
            <div class="goal-priority" title="Priority">${priorityDots}</div>
            ${g.payouts_needed > 0 ? `<span class="goal-payouts-needed">${g.payouts_needed} payouts away</span>` : ''}
            ${g.status === 'completed' ? '<span style="color:var(--green)">COMPLETED</span>' : ''}
            <div class="goal-actions">
              ${g.status === 'active' ? `<button class="btn btn-sm" onclick="event.stopPropagation(); App.contributeGoal('${g.id}', '${esc(g.name)}')">+ Fund</button>` : ''}
            </div>
          </div>
        </div>`;
    }).join('');
  },

  renderBilling() {
    const b = this.state.billing;
    if (b) {
      setText('billingTotal', '$' + fmt(b.total_expenses));
      setText('billingPaid', '$' + fmt(b.total_paid));
      setText('billingPending', '$' + fmt(b.total_pending));
      setText('billingCoverage', Math.round((b.trading_coverage || 0) * 100) + '%');
    }

    const el = document.getElementById('expenseList');
    if (!this.state.expenses.length) {
      el.innerHTML = '<div class="empty-state">No expenses tracked — add your bills</div>';
      return;
    }
    el.innerHTML = this.state.expenses.map(e => `
      <div class="expense-item">
        <span class="expense-name">${esc(e.name)}</span>
        <span class="expense-category">${esc(e.category)}</span>
        <span class="expense-amount">$${fmt(e.amount)}</span>
        <span class="expense-freq">${e.frequency}</span>
        ${e.auto_pay ? '<span class="expense-autopay">AUTO</span>' : ''}
        <div class="expense-actions">
          <button class="btn btn-sm" onclick="event.stopPropagation(); App.payExpense('${e.id}', '${esc(e.name)}', ${e.amount})" title="Record payment">Pay</button>
        </div>
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
        const views = ['throne', 'tactical', 'forge', 'performance', 'council', 'arsenal'];
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
      { icon: '⎔', label: 'Arsenal (Holdings + Wheel)', key: '6', action: () => this.switchView('arsenal') },
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

App.login = async function(e) {
  e.preventDefault();
  const pw = document.getElementById('loginPassword').value;
  const errEl = document.getElementById('loginError');
  try {
    await fetch('/api/v1/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pw }),
    }).then(r => {
      if (!r.ok) throw new Error('Invalid password');
      return r.json();
    });
    document.getElementById('loginOverlay').classList.add('hidden');
    App.bindNav();
    App.bindKeyboard();
    App.connectSSE();
    App.pollStatus();
    App.loadData();
    App.loadCouncil();
    App.loadHoldings();
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
