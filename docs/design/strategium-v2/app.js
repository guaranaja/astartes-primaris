// Strategium v2 — main app
const App = {
  state: {
    view: 'throne',
    selectedMarine: 'eversor-1',
    selectedJob: 'FJ-0480',
    chartRange: '1d',
    cmdOpen: false,
    tweaksOpen: false,
  },

  init() {
    this.renderAll();
    this.wireEvents();
    this.startClock();
    this.startLive();
    this.applyTweaks();
    this.wireEditMode();
  },

  wireEditMode() {
    window.addEventListener('message', (e) => {
      if (!e.data) return;
      if (e.data.type === '__activate_edit_mode') {
        document.getElementById('tweaks').classList.remove('hidden');
      } else if (e.data.type === '__deactivate_edit_mode') {
        document.getElementById('tweaks').classList.add('hidden');
      }
    });
    try { window.parent.postMessage({ type: '__edit_mode_available' }, '*'); } catch (e) {}
  },

  wireEvents() {
    document.querySelectorAll('.nav-btn').forEach(b => {
      b.addEventListener('click', () => this.switchView(b.dataset.view));
    });
    document.addEventListener('keydown', e => {
      if (e.ctrlKey && e.key === 'p') { e.preventDefault(); this.toggleCmd(true); }
      if (e.key === 'Escape') { this.toggleCmd(false); this.closeModal(); }
      if (e.ctrlKey && e.key === 'k') { e.preventDefault(); this.killSwitch(); }
      const n = parseInt(e.key);
      if (!e.ctrlKey && !e.altKey && n >= 1 && n <= 7 && document.activeElement.tagName !== 'INPUT') {
        const views = ['throne','tactical','forge','performance','council','prop','arsenal'];
        this.switchView(views[n-1]);
      }
    });
    document.getElementById('cmdp-backdrop').addEventListener('click', e => {
      if (e.target.id === 'cmdp-backdrop') this.toggleCmd(false);
    });
    document.getElementById('modal-backdrop').addEventListener('click', e => {
      if (e.target.id === 'modal-backdrop') this.closeModal();
    });
    document.getElementById('kill-btn').addEventListener('click', () => this.killSwitch());
    document.getElementById('cmd-btn').addEventListener('click', () => this.toggleCmd(true));
  },

  switchView(v) {
    this.state.view = v;
    document.querySelectorAll('.nav-btn').forEach(b => b.classList.toggle('active', b.dataset.view === v));
    document.querySelectorAll('.view').forEach(el => el.classList.toggle('active', el.id === 'view-'+v));
  },

  startClock() {
    const tick = () => {
      const d = new Date();
      const nyH = (d.getUTCHours() - 4 + 24) % 24;
      const nyM = d.getMinutes();
      const nyS = d.getSeconds();
      const pad = n => String(n).padStart(2,'0');
      document.getElementById('clock-main').textContent = `${pad(nyH)}:${pad(nyM)}:${pad(nyS)}`;
    };
    tick(); setInterval(tick, 1000);
  },

  startLive() {
    // periodic fake live updates
    setInterval(() => {
      // pulse warmup on a random marine
      const m = Data.marines[Math.floor(Math.random() * Data.marines.length)];
      if (m.status === 'active' || m.status === 'dormant') {
        m.warmup = Math.max(0, Math.min(100, m.warmup + (Math.random() - 0.4) * 12));
        if (this.state.view === 'tactical') this.renderTactical();
        if (this.state.view === 'throne') this.renderEventFeed();
      }
    }, 2500);
  },

  renderAll() {
    this.renderTicker();
    this.renderThrone();
    this.renderTactical();
    this.renderForge();
    this.renderPerformance();
    this.renderCouncil();
    this.renderProp();
    this.renderArsenal();
    this.renderCmd();
  },

  renderTicker() {
    const track = document.getElementById('ticker-track');
    const html = Data.tickers.map(t => {
      const cls = t.chg >= 0 ? 'pos' : 'neg';
      const sign = t.chg >= 0 ? '+' : '';
      return `<span class="tkr">
        <span class="tkr-sym">${t.sym}</span>
        <span class="tkr-px">${t.px.toLocaleString(undefined, {minimumFractionDigits: t.sym==='BTC'||t.sym==='VIX'?2:2})}</span>
        <span class="tkr-chg ${cls}">${sign}${t.chg.toFixed(2)} ${sign}${t.pct.toFixed(2)}%</span>
      </span>`;
    }).join('');
    track.innerHTML = html + html;
  },

  // ─── THRONE ──────────────────────────────────────────────
  renderThrone() {
    this.renderKpis();
    this.renderFortresses();
    this.renderEventFeed();
    this.renderRiskMatrix();
    this.renderEquityMini();
  },

  renderKpis() {
    const totalPnl = Data.fortresses.reduce((s,f) => s+f.pnlDay, 0);
    const totalCap = Data.fortresses.reduce((s,f) => s+f.capital, 0);
    const active = Data.marines.filter(m => m.status === 'active').length;
    const failed = Data.marines.filter(m => m.status === 'failed').length;
    const waking = Data.marines.filter(m => m.status === 'waking').length;
    const dayTrades = 42, wins = 28;
    const payouts = Data.payouts.reduce((s,p) => s+p.net, 0);

    const kpis = [
      { label: "Today's P&L", value: this.money(totalPnl), pos: totalPnl>=0, sub: `${(totalPnl/totalCap*100).toFixed(2)}% of capital`, primary: true, spark: Data.rndSpark(40, 1) },
      { label: "Capital AUM", value: this.money(totalCap, 0), sub: `across ${Data.fortresses.length} fortresses`, spark: Data.rndSpark(40, 0.3) },
      { label: "Marines Live", value: `${active}/${Data.marines.length}`, sub: `${waking} waking · ${failed} failed`, warn: failed>0 },
      { label: "Trades Today", value: dayTrades, sub: `${((wins/dayTrades)*100).toFixed(0)}% win · ${wins}W/${dayTrades-wins}L` },
      { label: "Lifetime Payouts", value: this.money(payouts, 0), sub: `${Data.payouts.length} transfers · ${Data.propAccounts.filter(a=>a.status==='active').length} funded`, pos: true },
      { label: "Risk Utilization", value: "34%", sub: "well below limits" },
    ];

    document.getElementById('kpi-strip').innerHTML = kpis.map((k,i) => {
      const cls = k.primary ? 'primary' : (k.pos ? 'pos' : k.neg ? 'neg' : k.warn ? 'warn' : '');
      const valCls = k.pos === true ? 'pos' : (k.pos === false ? 'neg' : '');
      return `<div class="kpi ${cls}">
        <div class="kpi-top">
          <div class="kpi-label">${k.label}</div>
          ${k.spark ? `<div class="kpi-trend ${k.pos?'pos':'neg'}">24h</div>` : ''}
        </div>
        ${k.spark ? `<svg class="kpi-spark" viewBox="0 0 60 24" preserveAspectRatio="none">${this.sparkPath(k.spark, 60, 24, k.pos?'var(--green)':'var(--red)')}</svg>` : ''}
        <div class="kpi-value ${valCls}">${k.value}</div>
        <div class="kpi-sub">${k.sub}</div>
      </div>`;
    }).join('');
  },

  renderFortresses() {
    const grid = document.getElementById('fortress-grid');
    grid.innerHTML = Data.fortresses.map(f => {
      const pnlCls = f.pnlDay >= 0 ? 'pos' : 'neg';
      const sign = f.pnlDay >= 0 ? '+' : '';
      return `<div class="fortress-card" data-class="${f.tag}">
        <div class="fortress-head">
          <div class="fortress-ident">
            <div class="fortress-numeral">${f.numeral}</div>
            <div>
              <div class="fortress-name">${f.name}</div>
              <div class="fortress-asset">${f.asset}</div>
            </div>
          </div>
          <div class="fortress-pnl">
            <div class="fortress-pnl-value ${pnlCls}">${sign}${this.money(f.pnlDay)}</div>
            <div class="fortress-pnl-pct ${pnlCls}">${sign}${f.pnlPct.toFixed(2)}%</div>
          </div>
        </div>
        <div class="fortress-body">
          <div class="fortress-stats">
            <div class="fstat"><span class="fstat-label">Capital</span><span class="fstat-value">${this.money(f.capital, 0)}</span></div>
            <div class="fstat"><span class="fstat-label">Marines</span><span class="fstat-value">${f.marines}</span></div>
            <div class="fstat"><span class="fstat-label">Active</span><span class="fstat-value" style="color:var(--green)">${f.active}</span></div>
            <div class="fstat"><span class="fstat-label">Companies</span><span class="fstat-value">${f.companies.length}</span></div>
          </div>
          ${f.companies.map(c => `<div class="company-row">
            <span class="company-badge">${c.id.toUpperCase()}</span>
            <span class="company-name">${c.name}</span>
            <span class="company-pills">${c.marines.split('').map(s => {
              const m = {A:'active',D:'dormant',F:'failed'}[s] || 'disabled';
              return `<span class="cpill ${m}"></span>`;
            }).join('')}</span>
          </div>`).join('')}
        </div>
      </div>`;
    }).join('');
  },

  renderEventFeed() {
    document.getElementById('event-feed').innerHTML = Data.events.slice(0, 14).map(e => `
      <div class="event-item">
        <div class="event-time">${e.t}</div>
        <div class="event-icon ${e.ic}">${this.evIcon(e.ic)}</div>
        <div class="event-msg">${e.msg}</div>
        <div class="event-tag">${e.tag}</div>
      </div>`).join('');
    document.getElementById('event-count').textContent = Data.events.length;
  },

  renderRiskMatrix() {
    const cells = [
      { l: 'Daily Loss', v: '-$1.1k', pct: 22, lim: '$5k' },
      { l: 'Max Position', v: '12 ES', pct: 60, lim: '20 ES', cls: 'warn' },
      { l: 'Open Risk', v: '$2.4k', pct: 48, lim: '$5k' },
      { l: 'Concentration', v: '34%', pct: 34, lim: '50%' },
      { l: 'Correlation', v: '0.42', pct: 52, lim: '0.80' },
      { l: 'Leverage', v: '2.1x', pct: 42, lim: '5.0x' },
    ];
    document.getElementById('risk-matrix').innerHTML = cells.map(c => `
      <div class="risk-cell ${c.cls||''}">
        <div class="risk-cell-label">${c.l}</div>
        <div class="risk-cell-val">${c.v}</div>
        <div class="risk-cell-meter"><div class="risk-cell-meter-fill" style="width:${c.pct}%"></div></div>
        <div class="risk-cell-label" style="margin-top:2px;color:var(--text-3)">lim ${c.lim}</div>
      </div>`).join('');
  },

  renderEquityMini() {
    const pts = Data.rndSpark(80, 1);
    const svg = this.sparkPath(pts, 300, 80, 'var(--green)', true);
    const cur = 942840, chg = 4287.50;
    document.getElementById('equity-mini').innerHTML = `
      <div class="equity-mini-header">
        <div>
          <div class="kpi-label">Imperium Equity</div>
          <div class="equity-mini-value pos">${this.money(cur, 0)}</div>
        </div>
        <div class="equity-mini-chg pos">+${this.money(chg)} today · +0.46%</div>
      </div>
      <div class="equity-mini-chart"><svg viewBox="0 0 300 80" preserveAspectRatio="none" style="width:100%;height:100%">${svg}</svg></div>`;
  },

  // ─── TACTICAL ────────────────────────────────────────────
  renderTactical() {
    this.renderMarineList();
    this.renderGauges();
    this.renderMarineDetail();
    this.renderTape();
  },

  renderMarineList() {
    document.getElementById('marine-list').innerHTML = Data.marines.map(m => {
      const sign = m.pnl >= 0 ? '+' : '';
      const pnlCls = m.pnl >= 0 ? 'pos' : 'neg';
      return `<div class="marine-row ${m.id === this.state.selectedMarine ? 'selected' : ''}" data-id="${m.id}" onclick="App.selectMarine('${m.id}')">
        <span class="sdot ${m.status}"></span>
        <div class="marine-info">
          <div class="marine-name">${m.name} <span class="marine-chapter">${m.chapter}</span></div>
          <div class="marine-detail"><span class="sym">${m.sym}</span><span>${m.strat}</span><span>${m.schedule}</span></div>
        </div>
        <div class="marine-pnl ${pnlCls}">
          ${m.pnl === 0 ? '—' : sign + this.money(m.pnl)}
          <span class="pnl-sub">${m.pos.slice(0, 14)}</span>
        </div>
      </div>`;
    }).join('');
  },

  renderGauges() {
    const live = Data.marines.filter(m => m.status !== 'disabled' && m.warmup > 0);
    document.getElementById('gauge-grid').innerHTML = live.map(m => {
      let state = 'cold';
      if (m.warmup >= 80) state = 'signal';
      else if (m.warmup >= 60) state = 'hot';
      else if (m.warmup >= 35) state = 'warm';
      return `<div class="gauge ${state}">
        <div class="gauge-top">
          <div class="gauge-label">${m.chapter.slice(0,4)}-${m.id.split('-').pop()}</div>
          <div class="gauge-sym">${m.sym}</div>
        </div>
        <div class="gauge-value">${Math.round(m.warmup)}<span style="font-size:11px;opacity:0.6">%</span></div>
        <div class="gauge-bar"><div class="gauge-fill" style="width:${m.warmup}%"></div></div>
        <div class="gauge-markers"><span>COLD</span><span>WARM</span><span>HOT</span><span>FIRE</span></div>
      </div>`;
    }).join('');
  },

  renderMarineDetail() {
    const m = Data.marines.find(x => x.id === this.state.selectedMarine) || Data.marines[0];
    const hist = [];
    for (let i=0; i<40; i++) {
      const r = Math.random();
      hist.push(r > 0.92 ? 'fail' : r > 0.85 ? 'skip' : 'ok');
    }
    const sign = m.pnl >= 0 ? '+' : '';
    const pnlCls = m.pnl >= 0 ? 'pos' : 'neg';
    document.getElementById('marine-detail').innerHTML = `
      <div class="panel-header">
        <div class="panel-title"><span class="rune">◈</span> ${m.name}</div>
        <div class="panel-actions">
          <button class="btn btn-xs btn-success">WAKE</button>
          <button class="btn btn-xs btn-danger">KILL</button>
        </div>
      </div>
      <div class="detail-section">
        <div class="detail-label">Identity</div>
        <div style="font-family:var(--font-mono);font-size:13px;color:var(--text-0);font-weight:600">${m.id}</div>
        <div class="detail-grid" style="margin-top:6px">
          <div><div class="detail-label">Chapter</div><div style="color:var(--gold);font-family:var(--font-mono);font-size:11px">${m.chapter}</div></div>
          <div><div class="detail-label">Fortress</div><div style="color:var(--ultra-bright);font-family:var(--font-mono);font-size:11px">${m.fortress.toUpperCase()}</div></div>
          <div><div class="detail-label">Strategy</div><div style="font-family:var(--font-mono);font-size:11px">${m.strat}</div></div>
          <div><div class="detail-label">Schedule</div><div style="font-family:var(--font-mono);font-size:11px">${m.schedule}</div></div>
        </div>
      </div>
      <div class="detail-section">
        <div class="detail-label">Day P&L</div>
        <div class="detail-value ${pnlCls}" style="font-size:20px;font-weight:700">${m.pnl===0?'$0.00':sign+this.money(m.pnl)}</div>
      </div>
      <div class="detail-section">
        <div class="detail-label">Position</div>
        <div class="position-card ${m.pos === 'FLAT' || m.pos === 'PENDING' ? 'flat' : 'open'}">
          <div style="font-family:var(--font-mono);font-size:12px;color:var(--text-0);font-weight:600">${m.pos}</div>
          ${m.pos !== 'FLAT' && m.pos !== 'PENDING' && m.pos !== 'STALE' ? `
            <div style="display:flex;justify-content:space-between;margin-top:4px;font-family:var(--font-mono);font-size:10px;color:var(--text-2)">
              <span>unrealized</span><span class="${pnlCls}">${sign}${this.money(Math.abs(m.pnl)*0.6)}</span>
            </div>` : ''}
        </div>
      </div>
      <div class="detail-section">
        <div class="detail-label">Last 40 Cycles</div>
        <div class="cycle-history">${hist.map(h => `<div class="cycle-tick ${h}" style="height:${h==='ok'?20+Math.random()*8:h==='fail'?8:4}px" title="${h}"></div>`).join('')}</div>
        <div style="display:flex;justify-content:space-between;margin-top:4px;font-family:var(--font-mono);font-size:9px;color:var(--text-3)">
          <span>${hist.filter(h=>h==='ok').length} OK</span>
          <span>${hist.filter(h=>h==='fail').length} FAIL</span>
          <span>${hist.filter(h=>h==='skip').length} SKIP</span>
        </div>
      </div>
      <div class="detail-section">
        <div class="detail-label">Next Cycle</div>
        <div style="font-family:var(--font-mono);font-size:11px;color:var(--amber)">in ${m.lastCycle}s · ${m.status.toUpperCase()}</div>
      </div>`;
  },

  renderTape() {
    const tape = [];
    const syms = ['ES','NQ','NVDA','AAPL','MSFT','SPY','META','TSLA','AMZN','GOOG'];
    for (let i=0; i<30; i++) {
      const s = syms[Math.floor(Math.random()*syms.length)];
      const side = Math.random() > 0.5 ? 'BUY' : 'SELL';
      const px = s==='ES' ? 5280 + Math.random()*10 : s==='NQ' ? 18340 + Math.random()*30 : 100 + Math.random()*500;
      const qty = Math.floor(1 + Math.random()*50);
      const t = `14:${String(32-Math.floor(i/4)).padStart(2,'0')}:${String(Math.floor(Math.random()*60)).padStart(2,'0')}`;
      const venues = ['APX#1','APX#2','TF#1','IBKR','TASTY','MFFU'];
      tape.push({ t, side, sym: s, px, qty, venue: venues[Math.floor(Math.random()*venues.length)] });
    }
    document.getElementById('tape').innerHTML = tape.map(r => `
      <div class="tape-row ${r.side.toLowerCase()}">
        <span class="tape-time">${r.t}</span>
        <span class="tape-sym">${r.sym}</span>
        <span class="tape-side">${r.side}</span>
        <span class="tape-px">${r.px.toFixed(2)}</span>
        <span class="tape-qty">${r.qty}</span>
        <span class="tape-venue">${r.venue}</span>
      </div>`).join('');
  },

  selectMarine(id) {
    this.state.selectedMarine = id;
    this.renderMarineList();
    this.renderMarineDetail();
  },

  // ─── FORGE ───────────────────────────────────────────────
  renderForge() {
    document.getElementById('forge-jobs-body').innerHTML = Data.backtestJobs.map(j => `
      <tr class="${j.id===this.state.selectedJob?'selected':''}" onclick="App.selectJob('${j.id}')">
        <td style="color:var(--gold)">${j.id}</td>
        <td style="color:var(--text-0)">${j.name}</td>
        <td><span class="job-status ${j.status}">${j.status}</span></td>
        <td><div class="job-progress"><div class="job-progress-bar"><div class="job-progress-fill ${j.status==='completed'?'done':j.status==='failed'?'fail':''}" style="width:${j.progress}%"></div></div><span>${j.progress}%</span></div></td>
        <td>${j.dur}</td>
        <td class="${j.cagr>0?'pos':j.cagr<0?'neg':''}">${j.cagr!=null?j.cagr.toFixed(1)+'%':'—'}</td>
        <td>${j.sharpe?j.sharpe.toFixed(2):'—'}</td>
        <td class="${j.dd?'neg':''}">${j.dd?j.dd.toFixed(1)+'%':'—'}</td>
      </tr>`).join('');
    this.renderForgeDetail();
  },

  selectJob(id) { this.state.selectedJob = id; this.renderForge(); },

  renderForgeDetail() {
    const j = Data.backtestJobs.find(x => x.id === this.state.selectedJob) || Data.backtestJobs[2];
    if (j.status !== 'completed') {
      document.getElementById('forge-detail').innerHTML = `<div class="panel-header"><div class="panel-title"><span class="rune">◈</span> Job Detail — ${j.id}</div></div>
        <div style="padding:40px;text-align:center;color:var(--text-3);font-family:var(--font-mono);font-size:11px">${j.status === 'running' ? 'Running — results will appear on completion' : j.status === 'queued' ? 'Queued — waiting for compute' : 'Job failed — check logs'}</div>`;
      return;
    }
    const pts = Data.rndSpark(120, j.cagr > 0 ? 1 : -1);
    const svg = this.sparkPath(pts, 500, 200, j.cagr > 0 ? 'var(--green)' : 'var(--red)', true);
    document.getElementById('forge-detail').innerHTML = `
      <div class="panel-header">
        <div class="panel-title"><span class="rune">◈</span> Job Detail — ${j.id} · ${j.name}</div>
        <div class="panel-actions">
          <button class="btn btn-xs">Export</button>
          <button class="btn btn-xs btn-gold">Deploy to Marine</button>
        </div>
      </div>
      <div class="forge-detail-layout">
        <div class="forge-chart">
          <div class="chart-header">
            <div class="kpi-label">Equity Curve (Forge simulated)</div>
            <div class="chart-value pos">+${j.cagr.toFixed(1)}% CAGR</div>
          </div>
          <div class="chart-svg-wrap"><svg viewBox="0 0 500 200" preserveAspectRatio="none" style="width:100%;height:100%">${svg}</svg></div>
        </div>
        <div class="forge-stats-col">
          <div class="stat-row"><span class="l">CAGR</span><span class="v pos">+${j.cagr.toFixed(2)}%</span></div>
          <div class="stat-row"><span class="l">Sharpe</span><span class="v">${j.sharpe.toFixed(2)}</span></div>
          <div class="stat-row"><span class="l">Sortino</span><span class="v">${(j.sharpe*1.4).toFixed(2)}</span></div>
          <div class="stat-row"><span class="l">Max DD</span><span class="v neg">${j.dd.toFixed(1)}%</span></div>
          <div class="stat-row"><span class="l">Calmar</span><span class="v">${(j.cagr/Math.abs(j.dd)).toFixed(2)}</span></div>
          <div class="stat-row"><span class="l">Win Rate</span><span class="v">62.4%</span></div>
          <div class="stat-row"><span class="l">Profit Factor</span><span class="v">2.14</span></div>
          <div class="stat-row"><span class="l">Trades</span><span class="v">1,284</span></div>
          <div class="stat-row"><span class="l">Avg Win</span><span class="v pos">+$142</span></div>
          <div class="stat-row"><span class="l">Avg Loss</span><span class="v neg">-$88</span></div>
          <div class="stat-row"><span class="l">Avg Hold</span><span class="v">4m 28s</span></div>
          <div class="stat-row"><span class="l">Exposure</span><span class="v">42%</span></div>
        </div>
      </div>`;
  },

  // ─── PERFORMANCE ─────────────────────────────────────────
  renderPerformance() {
    const pts = Data.rndSpark(180, 1);
    const cur = 942840, chg = 84200;
    document.getElementById('perf-equity').innerHTML = `
      <div class="panel-header">
        <div class="panel-title"><span class="rune">◈</span> Equity Curve — Imperium Aggregate</div>
        <div class="btn-group">
          ${['1D','1W','1M','3M','YTD','ALL'].map(r => `<button class="btn ${r==='1M'?'active':''}">${r}</button>`).join('')}
        </div>
      </div>
      <div class="chart-container">
        <div class="chart-header">
          <div>
            <div class="kpi-label">Portfolio Value</div>
            <div class="chart-value">${this.money(cur, 0)}</div>
          </div>
          <div style="text-align:right">
            <div class="kpi-label">30D Change</div>
            <div class="chart-value pos">+${this.money(chg, 0)} · +9.8%</div>
          </div>
        </div>
        <div class="chart-svg-wrap"><svg viewBox="0 0 800 220" preserveAspectRatio="none" style="width:100%;height:100%">${this.sparkPath(pts, 800, 220, 'var(--green)', true)}</svg></div>
      </div>`;

    const stats = [
      ['Total Return', '+$84,240', 'pos'],
      ['CAGR', '+42.8%', 'pos'],
      ['Sharpe Ratio', '2.14'],
      ['Sortino', '3.02'],
      ['Max Drawdown', '-8.4%', 'neg'],
      ['Calmar', '5.10'],
      ['Win Rate', '62.4%'],
      ['Profit Factor', '2.14'],
      ['Total Trades', '1,284'],
      ['Avg Win', '+$142', 'pos'],
      ['Avg Loss', '-$88', 'neg'],
      ['Avg Duration', '4m 28s'],
      ['Best Day', '+$4,842', 'pos'],
      ['Worst Day', '-$2,128', 'neg'],
      ['Profitable Days', '68%'],
      ['Volatility (1Y)', '12.4%'],
    ];
    document.getElementById('perf-stats-body').innerHTML = stats.map(([l,v,c]) => `
      <div class="stat-row"><span class="l">${l}</span><span class="v ${c||''}">${v}</span></div>`).join('');

    document.getElementById('journal-body').innerHTML = Data.journal.map(r => {
      const sign = r.pnl >= 0 ? '+' : '';
      return `<tr>
        <td>${r.t}</td>
        <td class="marine-col">${r.marine}</td>
        <td>${r.sig}</td>
        <td class="sym">${r.trade}</td>
        <td>${typeof r.entry === 'number' ? r.entry.toFixed(2) : r.entry}</td>
        <td>${typeof r.exit === 'number' ? r.exit.toFixed(2) : r.exit}</td>
        <td class="${r.pnl>0?'pos':r.pnl<0?'neg':''}">${r.pnl===0?'—':sign+this.money(r.pnl)}</td>
        <td>${r.reason}</td>
        <td>${r.dur}</td>
      </tr>`;
    }).join('');
  },

  // ─── COUNCIL ─────────────────────────────────────────────
  renderCouncil() {
    document.getElementById('phase-track').innerHTML = Data.phases.map(p => `
      <div class="phase-card ${p.status}">
        <div class="phase-rank">${p.rank} · ${p.goal}</div>
        <div class="phase-name">${p.name}</div>
        <div class="phase-goal">Target ${p.goal}</div>
        <div class="phase-milestones">
          ${p.milestones.map(m => `<div class="phase-milestone">
            <div class="phase-check ${m.done?'done':''}"></div>
            <span>${m.t}</span>
          </div>`).join('')}
        </div>
      </div>`).join('');

    document.getElementById('accounts-body').innerHTML = Data.accounts.map(a => `
      <div class="account-row">
        <span class="acct-badge ${a.badge.toLowerCase()}">${a.badge}</span>
        <div class="acct-main">
          <div class="acct-name">${a.name}</div>
          <div class="acct-detail">${a.firm} · ${a.num}${a.phase!=='--'?' · '+a.phase:''}</div>
        </div>
        <div class="acct-balance">
          <div class="acct-balance-val">${this.money(a.bal, 0)}</div>
          <div class="acct-balance-chg ${a.chg>=0?'pos':'neg'}">${a.chg>=0?'+':''}${this.money(a.chg)}</div>
        </div>
      </div>`).join('');

    document.getElementById('withdraw-list').innerHTML = Data.propAccounts.filter(a => a.status === 'active').slice(0,4).map(a => {
      let urg = 'hold', reason = 'accumulating profit';
      if (a.available >= 1500) { urg = 'now'; reason = `${a.days} days eligible · ${this.money(a.available)} available`; }
      else if (a.available >= 500) { urg = 'soon'; reason = `${a.days} days · near threshold`; }
      return `<div class="withdraw-card ${urg}">
        <div class="pip"></div>
        <div class="withdraw-body">
          <div class="acct">${a.num}</div>
          <div class="reason">${reason}</div>
        </div>
        <div class="withdraw-right">
          <div class="withdraw-amount">${this.money(a.available)}</div>
          <div class="withdraw-urgency ${urg}">${urg === 'now' ? 'WITHDRAW NOW' : urg === 'soon' ? 'SOON' : 'HOLD'}</div>
        </div>
      </div>`;
    }).join('');

    const totalExp = Data.expenses.reduce((s,e)=>s+e.amt, 0);
    const paid = Data.expenses.filter(e => e.autopay).reduce((s,e)=>s+e.amt, 0);
    const trading = 8240;
    document.getElementById('expense-stats').innerHTML = `
      <div class="expense-stat"><div class="expense-stat-label">Monthly Bills</div><div class="expense-stat-val">${this.money(totalExp, 0)}</div></div>
      <div class="expense-stat"><div class="expense-stat-label">Paid</div><div class="expense-stat-val pos">${this.money(paid, 0)}</div></div>
      <div class="expense-stat"><div class="expense-stat-label">Pending</div><div class="expense-stat-val">${this.money(totalExp-paid, 0)}</div></div>
      <div class="expense-stat"><div class="expense-stat-label">Coverage</div><div class="expense-stat-val pos">${Math.round(trading/totalExp*100)}%</div></div>`;
    document.getElementById('expense-list').innerHTML = Data.expenses.map(e => `
      <div class="expense-row">
        <span class="sdot ${e.autopay?'active':'dormant'}"></span>
        <span>${e.name}</span>
        <span class="expense-cat">${e.cat}</span>
        <span class="expense-freq">${e.freq.slice(0,3).toUpperCase()}</span>
        <span class="expense-amount">-${this.money(e.amt, 0)}</span>
      </div>`).join('');

    document.getElementById('alloc-list').innerHTML = Data.allocation.map(a => `
      <div class="alloc-row">
        <span class="alloc-label">${a.label}</span>
        <div class="alloc-bar"><div class="alloc-fill ${a.cls}" style="width:${a.pct*3}%"></div></div>
        <span class="alloc-pct">${a.pct}%</span>
      </div>`).join('');

    document.getElementById('goal-list').innerHTML = Data.goals.map(g => {
      const pct = (g.cur/g.tgt)*100;
      const done = pct >= 100;
      return `<div class="goal-card ${done?'completed':''} priority-${g.prio}">
        <div class="goal-head">
          <div class="goal-name">${g.name}</div>
          <div class="goal-eta">${g.eta}</div>
        </div>
        <div class="goal-bar"><div class="goal-fill" style="width:${Math.min(100,pct)}%"></div></div>
        <div class="goal-amounts">
          <span><span class="current">${this.money(g.cur, 0)}</span> <span class="target">/ ${this.money(g.tgt, 0)}</span></span>
          <span style="color:var(--gold)">${pct.toFixed(0)}%</span>
        </div>
      </div>`;
    }).join('');

    const metrics = [
      ['Lifetime P&L', '+$142,840', 'pos'],
      ['Lifetime Payouts', '$78,420', 'gold'],
      ['Monthly Net', '+$8,240', 'pos'],
      ['Portfolio Value', '$484,220'],
      ['Current Phase', 'BATTLE BROTHER', 'gold'],
      ['Goal Progress', '58%'],
      ['Profitable Days', '68%'],
      ['Accounts Closed', '3', 'neg'],
    ];
    document.getElementById('biz-metrics').innerHTML = metrics.map(([l,v,c]) => `
      <div class="metric"><div class="metric-label">${l}</div><div class="metric-val ${c==='pos'?'pos':c==='neg'?'neg':c==='gold'?'gold':''}">${v}</div></div>`).join('');
  },

  // ─── PROP ────────────────────────────────────────────────
  renderProp() {
    const active = Data.propAccounts.filter(a => a.status === 'active');
    const totalCap = active.reduce((s,a) => s + parseInt(a.size)*1000, 0);
    const totalPnl = active.reduce((s,a) => s + a.profit, 0);
    const totalAvail = active.reduce((s,a) => s + a.available, 0);
    const totalPaid = Data.payouts.reduce((s,p) => s + p.net, 0);
    const stats = [
      ['Prop Accounts', active.length, ''],
      ['Total Capital', this.money(totalCap, 0), ''],
      ['Total P&L', '+' + this.money(totalPnl, 0), 'green'],
      ['Available', this.money(totalAvail, 0), 'green'],
      ['Lifetime Payouts', this.money(totalPaid, 0), 'gold'],
      ['Avg Split', '96%', 'gold'],
    ];
    document.getElementById('prop-banner').innerHTML = stats.map(([l,v,c]) => `
      <div class="banner-stat">
        <div class="banner-label">${l}</div>
        <div class="banner-val ${c}">${v}</div>
      </div>`).join('');

    document.getElementById('prop-acct-list').innerHTML = Data.propAccounts.map(a => {
      const dailyUsed = Math.min(100, Math.abs(a.daily)/1500*100);
      const ddCls = a.trailingDD < 1500 ? 'warn' : '';
      return `<div class="prop-acct ${a.status}">
        <div class="prop-acct-head">
          <div class="prop-acct-ident">
            <span class="prop-acct-num">${a.num}</span>
            <span class="prop-acct-firm">${a.firm} ${a.size}</span>
          </div>
          <span class="prop-acct-status-pill ${a.status}">${a.status === 'danger' ? '⚠ EVAL' : a.status}</span>
        </div>
        <div class="prop-bal-grid">
          <div class="pbal"><span class="pbal-l">Balance</span><span class="pbal-v">${this.money(a.bal, 0)}</span></div>
          <div class="pbal"><span class="pbal-l">Profit</span><span class="pbal-v ${a.profit>=0?'pos':'neg'}">${a.profit>=0?'+':''}${this.money(a.profit, 0)}</span></div>
          <div class="pbal"><span class="pbal-l">Available</span><span class="pbal-v pos">${this.money(a.available, 0)}</span></div>
          <div class="pbal"><span class="pbal-l">Day P&L</span><span class="pbal-v ${a.daily>=0?'pos':'neg'}">${a.daily>=0?'+':''}${this.money(a.daily, 0)}</span></div>
        </div>
        ${a.status === 'active' || a.status === 'danger' ? `
        <div class="prop-rules">
          <div class="prop-rule"><span class="l">Trailing DD</span><span class="v">${this.money(a.trailingDD,0)} left</span><div class="rule-bar"><div class="rule-bar-fill ${ddCls}" style="width:${Math.min(100,a.trailingDD/2500*100)}%"></div></div></div>
          <div class="prop-rule"><span class="l">Daily used</span><span class="v">${Math.round(dailyUsed)}%</span><div class="rule-bar"><div class="rule-bar-fill ${dailyUsed>60?'warn':''}" style="width:${dailyUsed}%"></div></div></div>
          <div class="prop-rule"><span class="l">Days traded</span><span class="v">${a.days} days</span><div class="rule-bar"><div class="rule-bar-fill" style="width:${Math.min(100,a.days/30*100)}%"></div></div></div>
          <div class="prop-rule"><span class="l">Next payout</span><span class="v" style="color:var(--gold)">${a.nextPayout}</span><div></div></div>
        </div>` : ''}
      </div>`;
    }).join('');

    document.getElementById('payout-body').innerHTML = Data.payouts.map(p => `
      <tr>
        <td>${p.d}</td>
        <td class="sym">${p.acct}</td>
        <td>${this.money(p.gross, 0)}</td>
        <td>${p.split}</td>
        <td class="pos">${this.money(p.net, 0)}</td>
        <td>${p.dest}</td>
        <td><span class="job-status completed">${p.status}</span></td>
      </tr>`).join('');

    const propMetrics = [
      ['Monthly Gross', '$13,800', 'pos'],
      ['Monthly Net', '$12,590', 'pos'],
      ['Firm Fees YTD', '$1,248', 'neg'],
      ['Payouts This Month', '4'],
      ['Active Accounts', active.length],
      ['Closed Accounts', '2'],
      ['Best Account', 'APX-50K-001', 'gold'],
      ['Payout Rate', '72%'],
    ];
    document.getElementById('prop-metrics').innerHTML = propMetrics.map(([l,v,c]) => `
      <div class="metric"><div class="metric-label">${l}</div><div class="metric-val ${c==='pos'?'pos':c==='neg'?'neg':c==='gold'?'gold':''}">${v}</div></div>`).join('');
  },

  // ─── ARSENAL ─────────────────────────────────────────────
  renderArsenal() {
    document.getElementById('holdings-body').innerHTML = Data.holdings.map(h => {
      const totalCost = h.shares * h.cost;
      const totalVal = h.shares * h.last;
      const pl = totalVal - totalCost;
      const plPct = (pl/totalCost)*100;
      return `<tr>
        <td class="sym" style="font-size:12px;font-weight:700">${h.sym}</td>
        <td>${h.shares}</td>
        <td>${h.cost.toFixed(2)}</td>
        <td>${h.last.toFixed(2)}</td>
        <td>${this.money(totalVal, 0)}</td>
        <td class="${pl>=0?'pos':'neg'}">${pl>=0?'+':''}${this.money(pl, 0)} (${plPct>=0?'+':''}${plPct.toFixed(1)}%)</td>
        <td>${Math.floor(h.shares/100)}</td>
        <td style="color:var(--text-3)">${h.notes}</td>
      </tr>`;
    }).join('');

    document.getElementById('wheel-cycles').innerHTML = Data.wheelCycles.map(w => `
      <div class="wheel-cycle ${w.stage}">
        <div class="wheel-head">
          <span class="wheel-sym">${w.sym}</span>
          <span class="wheel-stage ${w.stage}">${w.stage.replace('_',' ')}</span>
        </div>
        <div class="wheel-details">
          <div class="wheel-detail"><div class="l">Strike</div><div class="v">$${w.strike}</div></div>
          <div class="wheel-detail"><div class="l">Expiry</div><div class="v">${w.exp}</div></div>
          <div class="wheel-detail"><div class="l">Shares</div><div class="v">${w.shares}</div></div>
          <div class="wheel-detail"><div class="l">DTE</div><div class="v">${w.days}d</div></div>
        </div>
        <div class="wheel-premium"><span class="l">Premium collected</span><span class="v">+${this.money(w.premium, 0)}</span></div>
      </div>`).join('');

    const analysis = [
      { sym: 'AAPL', rec: 'SELL COVERED CALL', strike: 190, exp: '05-03', premium: '$142', delta: 0.32, pop: '68%' },
      { sym: 'MSFT', rec: 'SELL COVERED CALL', strike: 430, exp: '05-03', premium: '$218', delta: 0.30, pop: '70%' },
      { sym: 'NVDA', rec: 'HOLD — IV too high', strike: '-', exp: '-', premium: '-', delta: 0, pop: '-' },
      { sym: 'GOOG', rec: 'SELL COVERED CALL', strike: 175, exp: '05-17', premium: '$96', delta: 0.28, pop: '72%' },
      { sym: 'AMZN', rec: 'SELL COVERED CALL', strike: 190, exp: '05-03', premium: '$84', delta: 0.25, pop: '75%' },
    ];
    document.getElementById('analysis-body').innerHTML = analysis.map(a => `
      <tr>
        <td class="sym" style="font-size:12px;font-weight:700">${a.sym}</td>
        <td style="color:${a.rec.startsWith('SELL')?'var(--green)':'var(--text-2)'};font-weight:600">${a.rec}</td>
        <td>${a.strike}</td>
        <td>${a.exp}</td>
        <td class="pos">${a.premium}</td>
        <td>${a.delta || '-'}</td>
        <td>${a.pop}</td>
      </tr>`).join('');
  },

  // ─── COMMAND PALETTE ─────────────────────────────────────
  renderCmd() {
    const items = [
      { section: 'Navigate', items: [
        { ic: '⚔', label: 'Command Throne', key: '1' },
        { ic: '◈', label: 'Tactical Display', key: '2' },
        { ic: '⚒', label: 'Forge Console', key: '3' },
        { ic: '⌖', label: 'Performance', key: '4' },
        { ic: '✚', label: 'Council', key: '5' },
        { ic: '$', label: 'Prop Desk', key: '6' },
        { ic: '⛨', label: 'Arsenal', key: '7' },
      ]},
      { section: 'Actions', items: [
        { ic: '▲', label: 'Wake all dormant marines', key: 'W' },
        { ic: '▼', label: 'Sleep all active marines', key: 'S' },
        { ic: '⊘', label: 'Kill switch — halt everything', key: 'Ctrl+K' },
        { ic: '+', label: 'New backtest', key: 'N' },
        { ic: '+', label: 'Add prop account', key: 'A' },
        { ic: '$', label: 'Record payout', key: 'P' },
      ]},
      { section: 'Search', items: [
        { ic: '◈', label: 'Search marines…', key: '' },
        { ic: '⚔', label: 'Search fortresses…', key: '' },
      ]},
    ];
    document.getElementById('cmdp-results').innerHTML = items.map(s => `
      <div class="cmdp-section">
        <div class="cmdp-section-label">${s.section}</div>
        ${s.items.map((it,i) => `<div class="cmdp-item ${s.section==='Navigate'&&i===0?'selected':''}">
          <span class="cmdp-item-icon">${it.ic}</span>
          <span class="cmdp-item-label">${it.label}</span>
          ${it.key?`<span class="cmdp-item-shortcut">${it.key}</span>`:''}
        </div>`).join('')}
      </div>`).join('');
  },

  toggleCmd(open) {
    this.state.cmdOpen = open;
    document.getElementById('cmdp-backdrop').classList.toggle('hidden', !open);
    if (open) setTimeout(() => document.getElementById('cmdp-input').focus(), 30);
  },

  closeModal() { document.getElementById('modal-backdrop').classList.add('hidden'); },

  killSwitch() {
    document.getElementById('modal-body').innerHTML = `
      <p style="color:var(--text-1);margin-bottom:12px">This will immediately <strong style="color:var(--red)">halt all active marines</strong> across the Imperium, cancel pending orders, and flatten open positions.</p>
      <div style="background:var(--bg-1);border:1px solid var(--red-dim);border-radius:4px;padding:12px;margin-bottom:16px;font-family:var(--font-mono);font-size:11px;color:var(--red)">
        ⊘ ${Data.marines.filter(m=>m.status==='active').length} active marines will be stopped<br>
        ⊘ ${Data.marines.filter(m=>m.pos!=='FLAT'&&m.pos!=='PENDING'&&m.pos!=='STALE').length} open positions will be flattened<br>
        ⊘ All pending orders will be cancelled
      </div>
      <div style="display:flex;gap:8px;justify-content:flex-end">
        <button class="btn" onclick="App.closeModal()">Cancel</button>
        <button class="btn btn-danger" onclick="App.closeModal()">Confirm Kill</button>
      </div>`;
    document.getElementById('modal-title').textContent = '⊘ EMERGENCY KILL';
    document.getElementById('modal-backdrop').classList.remove('hidden');
  },

  applyTweaks() {
    const density = localStorage.getItem('density') || 'normal';
    const fil = localStorage.getItem('filigree') || 'subtle';
    document.documentElement.dataset.density = density;
    document.documentElement.dataset.filigree = fil;
    document.querySelectorAll('[data-density-btn]').forEach(b => b.classList.toggle('active', b.dataset.densityBtn === density));
    document.querySelectorAll('[data-filigree-btn]').forEach(b => b.classList.toggle('active', b.dataset.filigreeBtn === fil));
  },

  setDensity(d) { localStorage.setItem('density', d); this.applyTweaks(); },
  setFiligree(f) { localStorage.setItem('filigree', f); this.applyTweaks(); },

  // ─── helpers ─────────────────────────────────────────────
  money(n, dec=2) {
    if (n == null) return '—';
    const abs = Math.abs(n);
    const sign = n < 0 ? '-' : '';
    if (abs >= 1e6) return `${sign}$${(abs/1e6).toFixed(2)}M`;
    return `${sign}$${abs.toLocaleString(undefined, {minimumFractionDigits: dec, maximumFractionDigits: dec})}`;
  },

  evIcon(k) {
    return { wake:'▲', sleep:'▼', signal:'◈', order:'→', fill:'●', fail:'✕', warn:'⚠' }[k] || '·';
  },

  sparkPath(pts, w, h, color='var(--green)', fill=false) {
    const min = Math.min(...pts), max = Math.max(...pts), range = max-min || 1;
    const step = w / (pts.length - 1);
    const d = pts.map((p,i) => `${i===0?'M':'L'}${i*step},${h - ((p-min)/range)*(h-4) - 2}`).join(' ');
    let out = '';
    if (fill) {
      out += `<defs><linearGradient id="grd-${color.replace(/[^a-z]/gi,'')}" x1="0" x2="0" y1="0" y2="1"><stop offset="0%" stop-color="${color}" stop-opacity="0.25"/><stop offset="100%" stop-color="${color}" stop-opacity="0"/></linearGradient></defs>`;
      out += `<path d="${d} L${w},${h} L0,${h} Z" fill="url(#grd-${color.replace(/[^a-z]/gi,'')})"/>`;
    }
    out += `<path d="${d}" fill="none" stroke="${color}" stroke-width="1.5"/>`;
    return out;
  },
};

document.addEventListener('DOMContentLoaded', () => App.init());
