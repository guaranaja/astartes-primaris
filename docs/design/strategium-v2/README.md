# Handoff: Strategium v2 — Imperium Command Center

## Overview

Strategium is a **single-operator command center** for a solo algo-trading business that runs many automated strategies ("marines") across multiple prop-firm and personal accounts, organized into asset-class groupings ("fortresses"). It unifies live control, backtesting, performance analysis, prop-desk accounting, and personal-finance/career tracking in a single dark cockpit UI.

The design uses **Warhammer 40K Space Marine metaphors as the vocabulary** — Fortresses (asset classes), Companies (strategy groupings), Marines (individual bot instances), Forge (backtesting), Council (business ops), Arsenal (manual holdings). This is a deliberate choice by the user; keep the metaphor.

## About the Design Files

The four files bundled in this handoff (`Strategium.html`, `app.js`, `data.js`, `style.css`) are **design references created as an HTML prototype**. They are not production code to ship as-is. They show intended look, information architecture, and behavior.

The task is to **recreate these designs in the target codebase's environment** (the `strategium-v2` repo on GitHub — Electron + React based on what we explored). Use the repo's existing patterns, component libraries, routing, and state management. Where the repo already has primitives (buttons, modals, data tables), extend those rather than reimplementing.

If parts of the target app don't exist yet, the HTML prototype is authoritative for visual + interaction design.

## Fidelity

**High-fidelity.** Exact colors, typography, spacing, and interactions are specified. Recreate pixel-perfectly in the target stack. The only "lofi" parts are:

- Chart data (SVG sparklines with random points) — replace with real equity curves from the backend
- Ticker symbols/prices — replace with live feed
- Mock events in the Vox Feed — replace with real event stream
- Everything else (layouts, spacing, colors, typography, copy, states) is final

## Design System

### Colors (exact hex)

```
/* Backgrounds — deep navy */
--bg-0:       #060a14   /* app background */
--bg-1:       #0c1222   /* topbar, modals */
--bg-2:       #131d30   /* secondary surfaces, inputs */
--bg-3:       #1c2a42   /* tertiary */
--bg-card:    #0e1628   /* panels */
--bg-card-2:  #0a1020   /* panel gradients */

/* Borders */
--border:        #1e3050
--border-light:  #2a4060
--border-gold:   #4a3a18

/* Text */
--text-0:  #e4e8f4   /* primary */
--text-1:  #a8b4cc   /* secondary */
--text-2:  #607090   /* tertiary */
--text-3:  #405070   /* muted / labels */

/* Accents — gold (Imperial) + ultramarine (primary) */
--gold:         #c9a84c
--gold-bright:  #e8c766
--gold-dim:     #7a6430
--gold-glow:    rgba(201, 168, 76, 0.25)

--ultra:        #3b82f6
--ultra-bright: #60a5fa
--ultra-dim:    #1d4ed8
--ultra-deep:   #0a1c44

/* Status */
--green:     #2dd4a0   /* profit, active, wins */
--green-dim: #1a8a60
--red:       #ef4444   /* loss, failed, kill */
--red-dim:   #991b1b
--amber:     #fbbf24   /* warn, warming up */
--amber-dim: #78530d
--purple:    #a78bfa
--blue:      #60a5fa
```

### Typography

Three font families, loaded from Google Fonts:

- **Inter** (400/500/600/700) — body copy, nav, general UI
- **JetBrains Mono** (400/500/600/700) — numbers, tickers, IDs, timestamps, all tabular data
- **Cinzel** (500/600/700) — display face for brand, panel titles, fortress names, phase names. Always uppercase with tracked letter-spacing (2–3px).

Base size: 13px. Numbers should always be mono. Labels are 8.5–10px uppercase with 1–1.5px letter-spacing, colored `--gold` or `--text-3` depending on hierarchy.

### Spacing / radii

- Base gap: `12px` (tweakable: compact 8px, spacious 16px)
- Radius: `4px` standard, `8px` for panels/modals
- Panels have 1px `--border` + 1px gold filigree line at top (`::before` gradient) when filigree tweak is on

### Shadows / glow

- Panel hover: `0 6px 24px rgba(0,0,0,0.3), 0 0 0 1px rgba(201, 168, 76, 0.1)`
- Gold text glow (brand, fortress numerals): `text-shadow: 0 0 12px var(--gold-glow)`
- Signal-firing gauge: animated box-shadow pulsing green
- Kill button hover: `box-shadow: 0 0 14px rgba(239, 68, 68, 0.4)`

## Shell / Chrome (present on every view)

### Topbar (48px)
Linear gradient navy (`#0d1424` → `#090f1e`) with a gold filigree gradient line below. From left:
1. **Brand block**: Ω glyph in a 28×28 square (radial gradient `#1a2848` → `#050a14`, 1px `--gold-dim` border), then two stacked lines: "STRATEGIUM" (Cinzel 15px 700 gold, tracked 3px) above "Astartes Primaris · v2.4" (8.5px tracked 2.4px `--text-3`).
2. **Nav tabs** (7): Throne, Tactical, Forge, Performance, Council, Prop Desk, Arsenal. Each has a numeric key pill (1–7). Active tab: `--gold-bright` text, subtle ultramarine gradient background, gold top-tick and border. Inactive: `--text-2`, transparent.
3. **Connection pill**: green dot + "VOX · 4 BROKERS LINKED" in 9.5px mono caps, rgba(45,212,160,0.08) bg, green-dim border.
4. **Clock**: NY time HH:MM:SS in gold-bright mono 13px, label "NY · MARKET OPEN" below in 8px `--text-3`. Separated by a left border.
5. **Command palette button**: 30px square, ◈ glyph, hover → gold.
6. **Kill switch**: red pill, "⊘ KILL" in red mono bold tracked 1.5px. Hover fills solid red with red glow.

### Ticker (26px)
Dark strip below topbar with auto-scrolling symbols (90s full cycle). Each entry: gold symbol, white price, green/red change. Gradient fades at both edges.

### Content area
`calc(100vh - 48px - 26px)`, overflow-y auto, 12px padding. Background has two subtle radial gradients (ultramarine top-left, gold bottom-right) over `--bg-0`.

## Views

### 1. Throne (Command Dashboard)

**Layout**: KPI strip (6 cards) full width, then 2-column grid (2fr / 1fr).

**KPI cards** (6, equal columns, 96px min height):
- Top border 2px in category color: gold (primary — Today's P&L), ultra-dim (default), green-dim (positive), red-dim (negative), amber-dim (warn).
- Label: 9.5px gold caps tracked 1.4px.
- Value: JetBrains Mono 22px 700. Green/red tint for P&L.
- Sub: 10px mono `--text-2`.
- Optional 24h sparkline bottom-right at 0.4 opacity.

Cards shown: Today's P&L / Capital AUM / Marines Live / Trades Today / Lifetime Payouts / Risk Utilization.

**Left column — Fortresses panel**: 2×2 grid of fortress cards. Each card:
- Top 2px gradient bar (color by asset class: ultramarine=futures, amber=options, green=equities, purple=crypto)
- Head: large Cinzel numeral (I, II, III, IV) in gold with glow and right border, then fortress name + asset-class tagline; right-aligned day P&L value + percent
- Stats row (4 cols): Capital, Marines, Active, Companies
- Company rows: small gold badge (1ST/2ND/SCT), company name, and 3–5 status pills (green active, grey dormant, red failed)
- Hover: lift 1px, gold border

**Right column** (stacked):
- **Imperium Equity** panel — big label, aggregate value in green mono 20px, +change inline, 80px-tall filled-area sparkline
- **Risk Matrix** — 2×3 grid of cells, each with label, mono value, 3px progress bar, "lim X" footer. Left border colored by severity (green ok, amber warn, red danger)
- **Vox Feed** — scrolling event list. Each row: 58px mono time / 20px glyph (▲ wake, ▼ sleep, ◈ signal, → order, ● fill, ✕ fail, ⚠ warn) / message / tag pill

### 2. Tactical

4-panel grid (1.2fr 1fr 1fr / auto auto). Marines panel spans left full height; gauges span top-right; detail + tape stacked bottom-right.

**Marines list** (left, full height):
Each row (14 + 1fr + auto): status dot (green active / amber pulsing waking / grey dormant / red failed / grey-disabled) + info block (name bold 12px + small gold chapter; then mono detail line with ultramarine symbol, strategy, schedule) + right-aligned day P&L (mono 12px green/red) with position sub in 9px grey.

Selected row: gold-gradient left wash, 2px gold left border.

**Gauges** (top right): auto-fill 140px grid. Each gauge: small gold caps label + ultra symbol / big mono value with % / 4px progress bar / marker labels COLD/WARM/HOT/FIRE. States:
- `cold` (<35): dim grey value, ultra-dim bar
- `warm` (35–60): amber tint, amber border
- `hot` (60–80): red tint, red border, red glow
- `signal` (≥80): green, pulsing box-shadow, green glow — "about to fire"

**Marine detail** (bottom-center): sections separated by hairlines — Identity grid / Day P&L (large mono) / Position card (green left border if open) / 40-cycle history bar sparkline (green OK / red fail / grey skip with varying heights) / Next cycle countdown.

**Tape** (bottom-right): condensed Time & Sales. Each row green/red left border, mono 10.5px grid (time/sym/side/px/qty/venue).

### 3. Forge (Backtesting)

2-column layout (320px form / rest).

**Left — New Forge Job form**: stacked labeled inputs (gold 9px caps labels). Inputs dark with `--gold-dim` focus. Sections: strategy template (select), symbols, start/end dates, capital/contracts, slippage/commission, compute tier (3-button segmented LOCAL / CLUSTER-8 / CLUSTER-32), giant "⚒ IGNITE FORGE" gold button.

**Right (stacked)**:
- **Jobs queue table** (max 320px scroll): ID (gold mono) / Strategy / Status pill (running=amber, completed=green, failed=red, queued=grey) / Progress bar+% / Duration / CAGR (colored) / Sharpe / Max DD (red). Row click highlights gold.
- **Job detail panel**: header + two-column body — equity curve SVG (filled area, 500×200, green or red per result) with header showing CAGR big and gold "Deploy to Marine" button top-right; right sidebar is a list of stat rows with 2px gold left border each: CAGR / Sharpe / Sortino / Max DD / Calmar / Win Rate / Profit Factor / Trades / Avg Win / Avg Loss / Avg Hold / Exposure.

Queued/running/failed jobs show an empty-state message instead of the detail body.

### 4. Performance

Grid: 1fr 280px / auto auto. Equity chart top-left, stats top-right, full-width journal below.

**Equity panel**: header with Cinzel title and range buttons (1D/1W/1M/3M/YTD/ALL, 1M active). Body: current value + 30d delta, 240px filled SVG curve.

**Perf stats**: same "stat row" format as Forge detail — 16 rows covering returns, sharpe/sortino, drawdown, trade statistics.

**Trade journal**: wide table, 340px scroll. Cols: Time / Marine (ultra) / Signal / Symbol (gold) / Entry / Exit / P&L (colored) / Reason / Dur. Filter buttons ALL/WINS/LOSSES + Export CSV.

### 5. Council (Career + Life ops)

**Phase track** full-width first: 6 horizontal cards showing career progression (recruit → initiate → battle brother → [current active, gold glow] → veteran → captain → primarch). Each card: phase rank + goal, big Cinzel name, milestones with checkboxes. States: completed (green-dim border, green rank), active (gold glow, gold border), locked (0.45 opacity).

**Then 2-column layout**:

Left column (3 panels stacked):
- **Accounts Ledger**: rows with colored badge (PROP amber / PERS blue / PAPER grey), name + firm · account# · phase, balance mono 13px, delta mono green/red
- **Withdraw Queue**: cards with left pip (green=now/amber=soon/grey=hold), account + reason, amount green + urgency label
- **Allocation by Strategy**: horizontal bars with uppercase labels + colored fills + percent

Right column:
- **Business Metrics**: 4×2 grid, 1px gold-dim dividers between cells (`gap:1px` on gold background trick). Each cell: gold caps label + big mono value.
- **Goals & Campaigns**: cards with priority color left border (gold=P3, ultra=P2, green=completed). Name + ETA, gold-gradient progress bar, current/target amounts mono.
- **Monthly Expenses**: 4-stat strip top (Monthly Bills / Paid / Pending / Coverage) + list rows each with active-dot (autopay), name, category caps, frequency abbreviation, red amount.

### 6. Prop Desk

**Banner** at top: glassy gold-tinted strip, 6 columns of label + mono value (Prop Accounts / Total Capital / Total P&L green / Available green / Lifetime Payouts gold / Avg Split gold).

**Then 2-column**:

Left: **Prop Accounts list**. Each account card has green/red/gold/grey left border based on status (active/danger/graduated/blown). Head: account number + firm+size label + status pill. 4-col balance grid (Balance / Profit / Available / Day P&L, all mono, green/red). Rules section (active/danger only): 4 rows with label / value / meter bar — Trailing DD, Daily used, Days traded, Next payout date.

Right:
- **Prop Metrics** (same 4×2 metric grid style)
- **Payout History** table (max 420px): Date / Account (gold) / Gross / Split / Net (green) / Destination / Status pill

### 7. Arsenal (Manual long equities + options wheel)

Stacked panels:
- **Long Equity Holdings** table: Symbol (gold bold) / Shares / Avg Cost / Last / Market Value / P&L colored / Lots / Notes
- **Wheel Cycles** grid (auto-fill 220px). Each cycle card has a top border colored by stage: ultramarine=cash_secured_put default, purple=selling_puts, green=selling_calls, amber=assigned. Body: large gold symbol + stage pill, 2×2 detail grid (Strike, Expiry, Shares, DTE), green premium-collected footer strip.
- **Wheel Analysis** table: recommendations for today (SELL COVERED CALL etc), Strike, Expiry, Premium (green), Delta, POP

## Global Interactions & Behavior

### Keyboard
- `1` – `7` switch views (disabled when focus is inside an input)
- `Ctrl+P` open command palette
- `Ctrl+K` trigger kill switch confirmation modal
- `Esc` close palette or modal

### Command palette (`#cmdp`)
540px card centered at 15vh, gold border + gold glow shadow. Top: ◈ prefix + autofocus text input with gold-dim bottom border. Results grouped in sections (Navigate / Actions / Search); each item has glyph, label, shortcut pill. Selected item: gold left wash + gold left border. Hover same.

### Kill switch modal
Centered 480px card, gold border. Red Cinzel header "⊘ EMERGENCY KILL". Body explains consequences, shows live counts of active marines / open positions / pending orders, Cancel + red Confirm buttons bottom-right.

### Live updates
Every 2.5s, a random marine's `warmup` value is perturbed; if Tactical is visible, gauges re-render; if Throne is visible, vox feed re-renders. Signal-firing gauges (≥80) pulse via CSS animation.

### Tweaks panel
Bottom-right 280px floating panel, hidden by default. Toggled by host via postMessage protocol (`__activate_edit_mode` / `__deactivate_edit_mode` messages; page announces `__edit_mode_available` on load). Two controls: Density (compact/normal/spacious — changes `--gap`) and Filigree (off/subtle — toggles the gold hairlines on panels). State in localStorage.

## State Management

Minimal app state — the prototype keeps it on a single `App.state` object:

```js
{
  view: 'throne' | 'tactical' | 'forge' | 'performance' | 'council' | 'prop' | 'arsenal',
  selectedMarine: string,   // id of currently-detailed marine
  selectedJob: string,      // id of currently-detailed forge job
  chartRange: '1d' | '1w' | '1m' | '3m' | 'ytd' | 'all',
  cmdOpen: boolean,
  tweaksOpen: boolean,
}
```

In a real app these become route params (view, selectedMarine, selectedJob) + ephemeral UI state. Ticker / marines / events / performance should come from the backend over websockets; KPIs derive client-side from the live data.

## Data model (from `data.js`)

Inspect `data.js` for canonical shapes. Key entities:

- `fortresses[]` — id, numeral (roman), name, asset, tag (futures/options/equities/crypto), capital, pnlDay, pnlPct, marines (count), active (count), companies[{id, name, marines:"AAAAD"}] where each letter is a status flag (A=active, D=dormant, F=failed, anything else=disabled)
- `marines[]` — id, name, chapter, fortress, sym, strat, schedule, status (active/dormant/waking/failed/disabled), pos (position string or "FLAT"/"PENDING"/"STALE"), pnl, warmup (0-100), lastCycle (seconds)
- `events[]` — t (HH:MM:SS), ic (wake/sleep/signal/order/fill/fail/warn), msg, tag
- `backtestJobs[]` — id (FJ-NNNN), name, status (running/completed/failed/queued), progress, dur, cagr, sharpe, dd
- `journal[]` — t, marine, sig, trade (symbol), entry, exit, pnl, reason, dur
- `phases[]` — 6 career phases with name, rank, goal, status (completed/active/locked), milestones[{t, done}]
- `accounts[]`, `propAccounts[]`, `payouts[]`, `expenses[]`, `allocation[]`, `goals[]`, `holdings[]`, `wheelCycles[]`

## Assets

No external image assets. All iconography is Unicode glyphs: ⚔ ◈ ⚒ ⌖ ✚ ⛨ $ ▲ ▼ → ● ✕ ⚠ · Ω ⊘. Replace with a proper icon system (e.g. Lucide) in production — keep the visual weight similar (thin, geometric).

Sparkline charts are inline SVG generated client-side; replace with a real charting lib (Recharts / uPlot / TradingView).

## Files in this handoff

- `Strategium.html` — page shell, all markup scaffolding, font loads, script includes
- `style.css` — full design system as CSS custom properties + component styles
- `app.js` — render functions per view, event wiring, live-update loop, helpers (`money`, `sparkPath`, `evIcon`)
- `data.js` — mock dataset

Read these in that order to understand the design. The CSS is the canonical source of truth for color, typography, and spacing tokens.

## Notes for implementation

- The design target is desktop (≥1400px comfortably, down to 1200px with degraded grids). Mobile is not a priority.
- Text density is intentionally high — this is a pro tool for an operator who lives in it. Don't pad; keep line-heights tight.
- Numbers are sacred: always mono, always right-aligned in tables, always colored green/red for P&L, always with explicit sign (+/-) for deltas.
- The Imperial/Gothic metaphor is the product's voice — don't sanitize copy to generic SaaS language. "Fortresses", "Marines", "Forge", "Kill switch", "Vox feed", "Primarch" are intentional.
