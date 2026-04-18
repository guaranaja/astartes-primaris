// Mock data for Strategium
const Data = {
  tickers: [
    { sym: 'ES',    px: 5284.25, chg:  12.50, pct:  0.24 },
    { sym: 'NQ',    px: 18342.75, chg: -28.25, pct: -0.15 },
    { sym: 'CL',    px: 78.42, chg:  0.84, pct:  1.08 },
    { sym: 'GC',    px: 2384.60, chg:  8.20, pct:  0.35 },
    { sym: 'SPY',   px: 528.14, chg:  1.22, pct:  0.23 },
    { sym: 'QQQ',   px: 448.92, chg: -0.68, pct: -0.15 },
    { sym: 'NVDA',  px: 882.40, chg: 14.80, pct:  1.71 },
    { sym: 'AAPL',  px: 188.24, chg: -0.82, pct: -0.43 },
    { sym: 'MSFT',  px: 418.62, chg:  2.44, pct:  0.59 },
    { sym: 'TSLA',  px: 174.28, chg: -3.42, pct: -1.92 },
    { sym: 'BTC',   px: 66842.00, chg: 842.00, pct: 1.27 },
    { sym: 'VIX',   px: 14.28, chg: -0.42, pct: -2.86 },
    { sym: 'DXY',   px: 104.82, chg: 0.18, pct: 0.17 },
  ],

  fortresses: [
    {
      id: 'primus', numeral: 'I', name: 'Fortress Primus', asset: 'Futures · ES/NQ/CL',
      tag: 'futures', capital: 425000, pnlDay: 4284.50, pnlPct: 1.01,
      marines: 8, active: 5,
      companies: [
        { id: '1st', name: '1st Co · Veterans (Live)',  marines: 'AAAAD' },
        { id: '2nd', name: '2nd Co · Battle (Scaling)', marines: 'ADDF' },
        { id: 'sct', name: 'Scout · Paper',             marines: 'DDDA' },
      ],
    },
    {
      id: 'secundus', numeral: 'II', name: 'Fortress Secundus', asset: 'Options · SPX/SPY',
      tag: 'options', capital: 248000, pnlDay: 1842.20, pnlPct: 0.74,
      marines: 5, active: 3,
      companies: [
        { id: '1st', name: '1st Co · Income Wheel',  marines: 'AAA' },
        { id: '2nd', name: '2nd Co · Spreads',       marines: 'ADF' },
      ],
    },
    {
      id: 'tertius', numeral: 'III', name: 'Fortress Tertius', asset: 'Equities · Long',
      tag: 'equities', capital: 184000, pnlDay: -482.40, pnlPct: -0.26,
      marines: 4, active: 2,
      companies: [
        { id: '1st', name: '1st Co · Value',   marines: 'AAD' },
        { id: '2nd', name: '2nd Co · Growth',  marines: 'DA' },
      ],
    },
    {
      id: 'quartus', numeral: 'IV', name: 'Fortress Quartus', asset: 'Crypto · BTC/ETH',
      tag: 'crypto', capital: 85000, pnlDay: 642.80, pnlPct: 0.76,
      marines: 3, active: 1,
      companies: [
        { id: 'sct', name: 'Scout · Experimental', marines: 'ADD' },
      ],
    },
  ],

  marines: [
    { id: 'eversor-1',    name: 'Eversor Alpha-1',  chapter: 'Eversor',     fortress: 'primus',   sym: 'ES',   strat: 'RTH Intraday',     schedule: '30s · 09:30-16:00',   status: 'active',   warmup: 72, pnl: 428.50,  pos: 'LONG 2 ES @ 5282.50',  lastCycle: 12 },
    { id: 'eversor-2',    name: 'Eversor Alpha-2',  chapter: 'Eversor',     fortress: 'primus',   sym: 'ES',   strat: 'RTH Intraday',     schedule: '30s · 09:30-16:00',   status: 'active',   warmup: 58, pnl: 682.00,  pos: 'LONG 3 ES @ 5281.75',  lastCycle: 18 },
    { id: 'fenris-1',     name: 'Fenris Beta-1',    chapter: 'Fenris',      fortress: 'primus',   sym: 'ES',   strat: 'Globex Overnight', schedule: '5m · 18:00-09:00',    status: 'dormant',  warmup: 24, pnl: 0,       pos: 'FLAT',                 lastCycle: 184 },
    { id: 'fenris-2',     name: 'Fenris Beta-2',    chapter: 'Fenris',      fortress: 'primus',   sym: 'NQ',   strat: 'Globex Overnight', schedule: '5m · 18:00-09:00',    status: 'dormant',  warmup: 18, pnl: 0,       pos: 'FLAT',                 lastCycle: 204 },
    { id: 'stormchaser-1',name: 'Stormchaser G-1',  chapter: 'Stormchaser', fortress: 'tertius',  sym: 'NVDA', strat: 'Momentum Breakout',schedule: '1m · 09:30-16:00',    status: 'active',   warmup: 88, pnl: 1842.40, pos: 'LONG 100 NVDA @ 878.20', lastCycle: 8 },
    { id: 'stormchaser-2',name: 'Stormchaser G-2',  chapter: 'Stormchaser', fortress: 'tertius',  sym: 'META', strat: 'Momentum Breakout',schedule: '1m · 09:30-16:00',    status: 'waking',   warmup: 42, pnl: 0,       pos: 'PENDING',              lastCycle: 2 },
    { id: 'phalanx-1',    name: 'Phalanx Delta-1',  chapter: 'Phalanx',     fortress: 'secundus', sym: 'SPY',  strat: 'Wheel / Income',   schedule: '1h · daily',          status: 'active',   warmup: 34, pnl: 648.00,  pos: 'SHORT 5P @ 520 · EXP 05/03', lastCycle: 1204 },
    { id: 'phalanx-2',    name: 'Phalanx Delta-2',  chapter: 'Phalanx',     fortress: 'secundus', sym: 'AAPL', strat: 'Covered Calls',    schedule: '4h · daily',          status: 'active',   warmup: 46, pnl: 142.00,  pos: 'SHORT 2C @ 190 · EXP 05/10', lastCycle: 842 },
    { id: 'corax-1',      name: 'Corax Epsilon-1',  chapter: 'Corax',       fortress: 'tertius',  sym: 'AAPL', strat: 'Mean Reversion',   schedule: '5m · 09:30-16:00',    status: 'failed',   warmup: 0,  pnl: -184.20, pos: 'STALE',                lastCycle: 240 },
    { id: 'medusa-1',     name: 'Medusa Zeta-1',    chapter: 'Medusa',      fortress: 'tertius',  sym: 'SPY',  strat: 'Systematic Trend', schedule: 'daily 09:45',         status: 'dormant',  warmup: 12, pnl: 0,       pos: 'FLAT',                 lastCycle: 14280 },
    { id: 'nocturne-1',   name: 'Nocturne Eta-1',   chapter: 'Nocturne',    fortress: 'tertius',  sym: 'BRK.B',strat: 'Value Swing',      schedule: 'weekly Mon',          status: 'disabled', warmup: 0,  pnl: 0,       pos: 'FLAT',                 lastCycle: 604800 },
    { id: 'scout-1',      name: 'Scout Experiment', chapter: 'Scout',       fortress: 'quartus',  sym: 'BTC',  strat: 'Perp Funding Arb', schedule: '15m · 24/7',          status: 'active',   warmup: 52, pnl: 282.40,  pos: 'LONG 0.2 BTC @ 66420', lastCycle: 48 },
  ],

  events: [
    { t: '14:32:14', ic: 'signal', msg: 'Eversor Alpha-1: entry signal triggered (warmup 92%)',         tag: 'EVE-α1' },
    { t: '14:32:08', ic: 'order',  msg: 'Phalanx Δ-1: SPY 520P credit $1.42 × 5 submitted',             tag: 'PHA-δ1' },
    { t: '14:31:52', ic: 'fill',   msg: 'Eversor Alpha-1: filled LONG 2 ES @ 5282.50',                  tag: 'EVE-α1' },
    { t: '14:31:24', ic: 'wake',   msg: 'Stormchaser G-2: container spinning up · cycle #4284',         tag: 'STO-γ2' },
    { t: '14:30:48', ic: 'fill',   msg: 'Stormchaser G-1: filled LONG 100 NVDA @ 878.20',               tag: 'STO-γ1' },
    { t: '14:30:14', ic: 'signal', msg: 'Stormchaser G-1: momentum breakout confirmed on 1m',           tag: 'STO-γ1' },
    { t: '14:29:52', ic: 'wake',   msg: 'Eversor Alpha-2: cycle #12884 started',                        tag: 'EVE-α2' },
    { t: '14:29:18', ic: 'sleep',  msg: 'Fenris Beta-1: cycle #9248 complete · no signal · sleep',      tag: 'FEN-β1' },
    { t: '14:28:44', ic: 'fail',   msg: 'Corax Epsilon-1: data staleness >60s · marine marked failed',  tag: 'COR-ε1' },
    { t: '14:28:12', ic: 'warn',   msg: 'Fortress Primus: correlation between α1/α2 = 0.82',            tag: 'SYS' },
    { t: '14:27:48', ic: 'fill',   msg: 'Phalanx Δ-2: AAPL 190C closed $0.08 · kept $134 premium',      tag: 'PHA-δ2' },
    { t: '14:27:18', ic: 'wake',   msg: 'Scout Experiment: BTC funding scan cycle #842',                tag: 'SCT-e1' },
    { t: '14:26:42', ic: 'signal', msg: 'Medusa Zeta-1: daily trend signal LONG SPY (weekly bar)',      tag: 'MED-ζ1' },
    { t: '14:26:04', ic: 'sleep',  msg: 'Scout Experiment: cycle complete · BTC funding 0.008%',        tag: 'SCT-e1' },
  ],

  backtestJobs: [
    { id: 'FJ-0482', name: 'Eversor ES · param sweep',    status: 'running',   progress: 62, dur: '4m 12s', cagr: null,   sharpe: null, dd: null },
    { id: 'FJ-0481', name: 'Stormchaser NVDA · walk-fwd', status: 'running',   progress: 34, dur: '2m 48s', cagr: null,   sharpe: null, dd: null },
    { id: 'FJ-0480', name: 'Phalanx SPY Wheel · 2Y',      status: 'completed', progress: 100,dur: '12m 04s', cagr: 28.4,  sharpe: 1.82, dd: 8.2 },
    { id: 'FJ-0479', name: 'Fenris NQ Overnight · 5Y',    status: 'completed', progress: 100,dur: '24m 18s', cagr: 42.8,  sharpe: 2.14, dd: 12.4 },
    { id: 'FJ-0478', name: 'Corax AAPL · mean-rev tune',  status: 'failed',    progress: 42, dur: '3m 08s', cagr: null,   sharpe: null, dd: null },
    { id: 'FJ-0477', name: 'Medusa SPY · systematic',     status: 'completed', progress: 100,dur: '8m 42s', cagr: 14.2,  sharpe: 1.24, dd: 6.8 },
    { id: 'FJ-0476', name: 'Stormchaser META · momentum', status: 'completed', progress: 100,dur: '10m 12s', cagr: -4.2, sharpe: -0.18, dd: 18.4 },
    { id: 'FJ-0475', name: 'Phalanx AAPL · covered calls',status: 'queued',    progress: 0,  dur: '—',      cagr: null,   sharpe: null, dd: null },
  ],

  journal: [
    { t: '14:28:42', marine: 'EVE-α1', sig: 'LONG', trade: 'ES 06-25', entry: 5280.25, exit: 5282.75, pnl: 250, reason: 'TP',    dur: '2m 18s' },
    { t: '14:18:04', marine: 'STO-γ1', sig: 'LONG', trade: 'NVDA',     entry: 876.40,  exit: 878.20,  pnl: 180, reason: 'signal',dur: '4m 12s' },
    { t: '13:48:22', marine: 'PHA-δ2', sig: 'SHORT',trade: 'AAPL 190C',entry: 0.48,    exit: 0.08,    pnl: 80,  reason: 'decay', dur: '2h 14m' },
    { t: '13:24:18', marine: 'EVE-α2', sig: 'LONG', trade: 'ES 06-25', entry: 5278.00, exit: 5275.50, pnl: -125,reason: 'SL',    dur: '8m 42s' },
    { t: '12:42:04', marine: 'STO-γ1', sig: 'LONG', trade: 'NVDA',     entry: 872.10,  exit: 876.40,  pnl: 430, reason: 'trail', dur: '12m 24s' },
    { t: '12:08:44', marine: 'EVE-α1', sig: 'SHORT',trade: 'ES 06-25', entry: 5284.00, exit: 5281.25, pnl: 275, reason: 'TP',    dur: '3m 48s' },
    { t: '11:42:18', marine: 'PHA-δ1', sig: 'SHORT',trade: 'SPY 520P', entry: 1.42,    exit: '—',     pnl: 0,   reason: 'open',  dur: '—' },
    { t: '11:04:28', marine: 'COR-ε1', sig: 'LONG', trade: 'AAPL',     entry: 188.42,  exit: 186.80,  pnl: -162,reason: 'SL',    dur: '24m 12s' },
    { t: '10:28:14', marine: 'EVE-α1', sig: 'LONG', trade: 'ES 06-25', entry: 5276.25, exit: 5279.50, pnl: 325, reason: 'TP',    dur: '5m 14s' },
    { t: '10:12:42', marine: 'MED-ζ1', sig: 'LONG', trade: 'SPY',      entry: 526.80,  exit: 528.14,  pnl: 268, reason: 'EOD',   dur: '2h 42m' },
    { t: '09:48:04', marine: 'STO-γ1', sig: 'LONG', trade: 'NVDA',     entry: 868.20,  exit: 872.10,  pnl: 390, reason: 'signal',dur: '8m 18s' },
    { t: '09:32:18', marine: 'EVE-α1', sig: 'LONG', trade: 'ES 06-25', entry: 5272.50, exit: 5275.00, pnl: 250, reason: 'TP',    dur: '2m 44s' },
  ],

  phases: [
    { rank: 'I',    name: 'INITIATE',        goal: '$10k saved', status: 'completed',
      milestones: [{ t: 'Open first prop account', done: true }, { t: 'First live trade', done: true }, { t: '$10k emergency fund', done: true }] },
    { rank: 'II',   name: 'NEOPHYTE',        goal: '$25k profit', status: 'completed',
      milestones: [{ t: '3 funded accounts', done: true }, { t: 'First payout', done: true }, { t: 'Consistency established', done: true }] },
    { rank: 'III',  name: 'BATTLE BROTHER',  goal: '$100k profit', status: 'active',
      milestones: [{ t: '5+ funded accounts', done: true }, { t: '$50k lifetime payouts', done: true }, { t: 'Cover bills from trading', done: true }, { t: '$100k cumulative profit', done: false }] },
    { rank: 'IV',   name: 'VETERAN',         goal: '$250k profit', status: 'locked',
      milestones: [{ t: '10+ funded accounts', done: false }, { t: 'Quit day job', done: false }, { t: 'Personal portfolio $500k', done: false }] },
    { rank: 'V',    name: 'CHAPTER MASTER',  goal: '$1M profit', status: 'locked',
      milestones: [{ t: '$1M cumulative payouts', done: false }, { t: 'Multi-asset diversified', done: false }] },
    { rank: 'VI',   name: 'PRIMARCH',        goal: 'FI / independence', status: 'locked',
      milestones: [{ t: 'Portfolio covers lifestyle 3x', done: false }, { t: 'Legacy strategy suite', done: false }] },
  ],

  accounts: [
    { badge: 'PROP', firm: 'Apex',     num: 'APX-50K-001',  phase: 'Funded',  bal: 52400, chg: 428 },
    { badge: 'PROP', firm: 'Apex',     num: 'APX-100K-002', phase: 'Funded',  bal: 104820, chg: 842 },
    { badge: 'PROP', firm: 'TopStep',  num: 'TSU-50K-003',  phase: 'Funded',  bal: 54120, chg: -182 },
    { badge: 'PROP', firm: 'MyFunded', num: 'MFF-100K-004', phase: 'Eval',    bal: 102840, chg: 642 },
    { badge: 'PERS', firm: 'IBKR',     num: 'U12345678',    phase: '--',      bal: 284000, chg: 1420 },
    { badge: 'PERS', firm: 'Fidelity', num: 'Z98765432',    phase: '--',      bal: 200220, chg: -184 },
    { badge: 'PAPER',firm: 'IBKR',     num: 'DU9876543',    phase: '--',      bal: 100000, chg: 284 },
  ],

  propAccounts: [
    { num: 'APX-50K-001',  firm: 'Apex',     size: '50K',  status: 'active',  bal: 52400,  profit: 2400, available: 1842, daily: 428, trailingDD: 2100, days: 12, nextPayout: 'in 2 days' },
    { num: 'APX-100K-002', firm: 'Apex',     size: '100K', status: 'active',  bal: 104820, profit: 4820, available: 3284, daily: 842, trailingDD: 1820, days: 18, nextPayout: 'ELIGIBLE' },
    { num: 'TSU-50K-003',  firm: 'TopStep',  size: '50K',  status: 'active',  bal: 54120,  profit: 4120, available: 2842, daily: -182, trailingDD: 1200, days: 24, nextPayout: 'ELIGIBLE' },
    { num: 'MFF-100K-004', firm: 'MyFunded', size: '100K', status: 'danger',  bal: 102840, profit: 2840, available: 0,    daily: -842, trailingDD: 820,  days: 8,  nextPayout: 'rule breach risk' },
    { num: 'APX-150K-005', firm: 'Apex',     size: '150K', status: 'graduated', bal: 154200, profit: 4200, available: 0, daily: 0, trailingDD: 0, days: 0, nextPayout: '—' },
    { num: 'FTM-50K-006',  firm: 'FTMO',     size: '50K',  status: 'blown',   bal: 0, profit: 0, available: 0, daily: 0, trailingDD: 0, days: 0, nextPayout: '—' },
  ],

  payouts: [
    { d: '04-28', acct: 'APX-50K-001',  gross: 2200, split: '90%',  net: 1980, dest: 'Primary checking', status: 'complete' },
    { d: '04-28', acct: 'APX-100K-002', gross: 3800, split: '90%',  net: 3420, dest: 'Primary checking', status: 'complete' },
    { d: '04-14', acct: 'TSU-50K-003',  gross: 2800, split: '100%', net: 2800, dest: 'Roth IRA',         status: 'complete' },
    { d: '04-14', acct: 'APX-50K-001',  gross: 1600, split: '90%',  net: 1440, dest: 'Goal · Deck',      status: 'complete' },
    { d: '03-28', acct: 'APX-100K-002', gross: 2400, split: '90%',  net: 2160, dest: 'Primary checking', status: 'complete' },
    { d: '03-14', acct: 'APX-50K-001',  gross: 1800, split: '90%',  net: 1620, dest: 'Primary checking', status: 'complete' },
    { d: '02-28', acct: 'TSU-50K-003',  gross: 2200, split: '100%', net: 2200, dest: 'Roth IRA',         status: 'complete' },
    { d: '02-14', acct: 'APX-100K-002', gross: 3200, split: '90%',  net: 2880, dest: 'Goal · Emergency', status: 'complete' },
  ],

  allocation: [
    { label: 'Bills',    pct: 32, cls: 'blue' },
    { label: 'Taxes',    pct: 22, cls: 'amber' },
    { label: 'Savings',  pct: 18, cls: 'green' },
    { label: 'Goals',    pct: 14, cls: 'gold' },
    { label: 'Trading',  pct: 10, cls: 'purple' },
    { label: 'Lifestyle',pct:  4, cls: 'red' },
  ],

  goals: [
    { name: 'Emergency Fund (6mo)', cur: 28000, tgt: 36000, eta: 'Aug 2025', prio: 3 },
    { name: 'New Deck',             cur:  4800, tgt: 12000, eta: 'Jun 2025', prio: 2 },
    { name: 'Quit Day Job Buffer',  cur: 48000, tgt: 120000,eta: 'Mar 2026', prio: 3 },
    { name: 'Personal Trading Acct',cur: 284000,tgt: 500000,eta: 'Dec 2026', prio: 2 },
    { name: 'Roth IRA Max',         cur:  7000, tgt:  7000, eta: 'done',     prio: 1 },
  ],

  expenses: [
    { name: 'Mortgage',          cat: 'housing',   amt: 2420, freq: 'monthly', autopay: true },
    { name: 'Property Tax',      cat: 'housing',   amt:  642, freq: 'monthly', autopay: true },
    { name: 'Auto Insurance',    cat: 'transport', amt:  184, freq: 'monthly', autopay: true },
    { name: 'Internet / Fiber',  cat: 'utilities', amt:   89, freq: 'monthly', autopay: true },
    { name: 'Electric',          cat: 'utilities', amt:  142, freq: 'monthly', autopay: false },
    { name: 'Data Feeds (Rithmic)',cat:'trading',  amt:  184, freq: 'monthly', autopay: true },
    { name: 'Bloomberg Lite',    cat: 'trading',   amt:   95, freq: 'monthly', autopay: true },
    { name: 'GCP / Primaris',    cat: 'trading',   amt:  248, freq: 'monthly', autopay: true },
    { name: 'Groceries',         cat: 'food',      amt:  842, freq: 'monthly', autopay: false },
    { name: 'Streaming bundle',  cat: 'lifestyle', amt:   62, freq: 'monthly', autopay: true },
  ],

  holdings: [
    { sym: 'AAPL',  shares: 200, cost: 142.40, last: 188.24, notes: 'Core · wheel eligible' },
    { sym: 'MSFT',  shares: 100, cost: 284.20, last: 418.62, notes: 'Core · wheel eligible' },
    { sym: 'NVDA',  shares: 100, cost: 420.00, last: 882.40, notes: 'Momentum' },
    { sym: 'GOOG',  shares: 100, cost: 128.40, last: 168.20, notes: 'Core' },
    { sym: 'AMZN',  shares: 100, cost: 142.80, last: 184.50, notes: 'Core' },
    { sym: 'META',  shares:  50, cost: 284.00, last: 482.40, notes: 'Momentum · not wheel' },
    { sym: 'SPY',   shares: 200, cost: 420.80, last: 528.14, notes: 'Index hedge' },
    { sym: 'BRK.B', shares:  50, cost: 324.00, last: 412.80, notes: 'Value anchor' },
  ],

  wheelCycles: [
    { sym: 'AAPL',  stage: 'selling_calls', strike: 190, exp: '05-03', shares: 200, days: 6,  premium: 284 },
    { sym: 'MSFT',  stage: 'selling_calls', strike: 430, exp: '05-10', shares: 100, days: 13, premium: 218 },
    { sym: 'SPY',   stage: 'selling_puts',  strike: 520, exp: '05-03', shares: 0,   days: 6,  premium: 142 },
    { sym: 'GOOG',  stage: 'assigned',      strike: 165, exp: '—',     shares: 100, days: 0,  premium: 92 },
  ],

  // helpers
  rndSpark(n, bias=1) {
    const pts = [];
    let v = 50;
    for (let i=0; i<n; i++) {
      v += (Math.random() - 0.5 + bias*0.06) * 4;
      pts.push(v);
    }
    return pts;
  },
};
