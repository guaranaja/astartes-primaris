# AURUM — Brand & Style Manual

> The visual identity of the Astartes Primaris platform.

---

## Brand Identity

**Name**: AURUM (Latin: gold)
**Tagline**: Command your Imperium.
**Personality**: Military precision meets modern fintech. Dense information, zero fluff.
Every pixel earns its place. The interface should feel like sitting in a command center —
powerful, confident, and immediate.

**Reference Points**:
- Bloomberg Terminal — information density, keyboard-first
- Linear — dark aesthetic, clean type, smooth interactions
- Grafana — real-time data visualization, modular panels
- vCenter — hierarchical management, status-at-a-glance

---

## Color System

### Core Palette

| Token            | Hex       | Usage                                    |
|------------------|-----------|------------------------------------------|
| `--bg-0`         | `#0a0a0f` | Page background, deepest layer           |
| `--bg-1`         | `#12121a` | Top bar, modals, elevated surfaces       |
| `--bg-2`         | `#1a1a26` | Cards, rows, interactive surfaces        |
| `--bg-3`         | `#222233` | Active states, hover backgrounds         |
| `--bg-card`      | `#14141e` | Primary card/section background          |
| `--border`       | `#2a2a3a` | Default borders                          |
| `--border-light` | `#3a3a4a` | Hover/focus borders                      |

### Text Hierarchy

| Token     | Hex       | Usage                                     |
|-----------|-----------|-------------------------------------------|
| `--text-0`| `#e8e8f0` | Primary text, headings, values            |
| `--text-1`| `#b0b0c0` | Secondary text, labels, descriptions      |
| `--text-2`| `#707088` | Tertiary text, metadata, timestamps       |
| `--text-3`| `#505068` | Disabled text, placeholder, muted info    |

### Accent Colors

| Token       | Hex       | Usage                                    |
|-------------|-----------|------------------------------------------|
| `--gold`    | `#d4a843` | Brand accent, primary actions, logo      |
| `--gold-dim`| `#8a7030` | Gold borders, subtle gold backgrounds    |
| `--green`   | `#2dd4a0` | Positive P&L, active/online status       |
| `--green-dim`| `#1a8a60`| Green borders, success backgrounds       |
| `--red`     | `#ef4444` | Negative P&L, errors, kill switch        |
| `--red-dim` | `#991b1b` | Red borders, danger backgrounds          |
| `--blue`    | `#60a5fa` | Marine identity, links, informational    |
| `--blue-dim`| `#1e40af` | Blue borders, selection highlight        |
| `--amber`   | `#fbbf24` | Warnings, pending states, "waking" phase |
| `--purple`  | `#a78bfa` | Trade count, analytics accent            |

### Signal Warmup Gradient

The warmup system uses progressive color states to show how close an indicator
is to generating a signal. This is the core visual language of the Tactical Display.

| State    | Condition   | Border Color | Fill Color | Glow                |
|----------|-------------|-------------|------------|---------------------|
| `cold`   | 0–35%       | `--border`  | `--blue-dim` | None               |
| `warm`   | 35–60%      | `--amber`   | `--amber`    | None               |
| `hot`    | 60–80%      | `--red`     | `--red`      | Subtle red glow    |
| `signal` | 80–100%     | `--green`   | `--green`    | Pulsing green glow |

**Rule**: The warmup states must feel like watching a gauge build pressure.
Cold is ambient. Warm draws attention. Hot demands it. Signal pulses with authority.

---

## Typography

### Fonts

| Role      | Font Stack                                                    | Usage                          |
|-----------|---------------------------------------------------------------|--------------------------------|
| **Sans**  | `-apple-system, BlinkMacSystemFont, 'Inter', 'Segoe UI', sans-serif` | UI labels, navigation, prose |
| **Mono**  | `'SF Mono', 'Cascadia Code', 'JetBrains Mono', 'Fira Code', Consolas, monospace` | Values, data, code, prices |

### Sizing

| Element           | Size   | Weight | Case        | Tracking    |
|-------------------|--------|--------|-------------|-------------|
| Logo text         | 14px   | 700    | UPPERCASE   | 2px         |
| Section headers   | 13px   | 600    | UPPERCASE   | 1px         |
| Card labels       | 11px   | 400    | UPPERCASE   | 1px         |
| Card values       | 28px   | 700    | Normal      | —           |
| Body text         | 13px   | 400    | Normal      | —           |
| Metadata/stamps   | 10px   | 400    | Normal/UPPER| 0.5–1px     |
| Data table cells  | 11px   | 400    | Normal      | —           |
| Keyboard hints    | 10px   | 400    | Normal      | —           |

### Rules

- **ALL labels and section headers are uppercase with letter-spacing.** This creates
  the military/terminal feel without sacrificing readability.
- **ALL numerical values use the monospace font.** Prices, P&L, percentages, counts —
  everything with a number gets monospace. This ensures columns align and numbers
  feel precise.
- **Never use font sizes below 10px.** Readability is non-negotiable.

---

## Component Language

### Cards

Cards are the primary information container. They have a dark background (`--bg-card`),
subtle border, and rounded corners (`--radius-lg: 10px`).

- **Padding**: 16px–20px
- **Border**: 1px solid `--border`
- **Hover**: border transitions to `--border-light`
- **No shadows** — depth is communicated through background shade, not drop shadows

### Buttons

| Variant     | Background     | Border         | Text          | Usage                    |
|-------------|---------------|----------------|---------------|--------------------------|
| Default     | `--bg-3`      | `--border`     | `--text-1`    | Secondary actions        |
| Primary     | `--gold-dim`  | `--gold`       | `--gold`      | Primary CTA              |
| Danger      | `--red-dim`   | `--red`        | `--red`       | Destructive actions      |
| Success     | `--green-dim` | `--green`      | `--green`     | Positive confirmations   |

**Hover behavior**: Primary fills with `--gold`, text goes to `--bg-0` (inverts).
All others brighten slightly. Never use box-shadow on hover — use border glow.

**Sizing**:
- `btn-sm`: 4px 10px, 11px font
- Default: 6px 14px, 12px font
- `btn-lg`: 10px 20px, 13px font

### Status Indicators

The dot indicator system is used across all views:

| Status     | Color       | Effect              | Meaning                  |
|------------|-------------|----------------------|--------------------------|
| `dormant`  | `--text-3`  | Static               | Sleeping between cycles  |
| `active`   | `--green`   | Glow                 | Currently executing      |
| `waking`   | `--amber`   | Pulse animation       | Container spinning up    |
| `failed`   | `--red`     | Static               | Last cycle failed        |
| `disabled` | `--text-3`  | 40% opacity          | Kill switch / manual off |

**Rule**: Status dots are always 8px diameter with 50% border-radius.
Active and waking states use `box-shadow` for glow. The pulse animation is
1s infinite ease, oscillating opacity between 1.0 and 0.4.

### Keyboard Hints

Navigation hints appear as small bordered squares (18x18px) next to their label.
Background: `--bg-2`. Border: `--border`. Font: 10px mono.

When the associated nav item is active, the hint border and text switch to gold
(`--gold-dim` border, `--gold` text).

---

## Layout Principles

### Information Density

AURUM favors density over whitespace. This is a professional tool, not a marketing page.

- **Gap**: 12px between all elements (cards, sections, grid items)
- **Padding**: 16px inside sections and cards
- **No hero sections, no splashes, no welcome screens**
- The dashboard should show useful data within 100ms of loading

### Grid System

| View          | Grid                                    |
|---------------|-----------------------------------------|
| Status cards  | 4 columns, equal width                  |
| Fortress grid | Auto-fill, min 340px per card           |
| Tactical      | 2 columns (marines | gauges)            |
| Forge         | 380px form + fluid results              |
| Performance   | Fluid chart + 280px stats sidebar       |

### Scrolling

- The top bar is fixed (44px height)
- Main content scrolls vertically
- Individual sections (event feed, tables) have internal scroll
- **Custom scrollbar**: 6px wide, rounded, `--border` color

---

## Animation Guidelines

AURUM uses **minimal, purposeful animation**. Nothing decorative.

| Animation    | Duration | Easing     | Usage                          |
|-------------|----------|------------|--------------------------------|
| Hover       | 150ms    | `ease`     | Button/card border transitions |
| Fade in     | 200ms    | `ease`     | New event feed items           |
| Pulse       | 1s       | `infinite` | Waking/pending status dots     |
| Signal glow | 1.5s     | `infinite` | Signal-state gauge pulsing     |
| Gauge fill  | 500ms    | `ease`     | Warmup bar width changes       |

**Rules**:
- No transitions longer than 500ms for interactive elements
- No entrance animations on page load (data should just appear)
- Pulse animations are reserved for "attention needed" states
- Never animate color changes on text — only on borders and backgrounds

---

## Naming Convention

### CSS Classes

Follow BEM-lite naming:

```
.fortress-card          — Block
.fortress-header        — Element
.fortress-card:hover    — State via pseudo-class
.marine-pill.active     — State via modifier class
```

Prefix rules:
- `card-` for status cards (card-pnl, card-marines)
- `marine-` for marine-related elements
- `fortress-` for fortress-related elements
- `gauge-` for signal warmup gauges
- `forge-` for backtest/optimization elements
- `perf-` for performance view elements
- `btn-` for button variants
- `nav-` for navigation elements

### JavaScript

- `App` is the single global namespace
- Methods use camelCase: `App.switchView()`, `App.wakeMarine()`
- State lives in `App.state` — single source of truth
- DOM updates are explicit render calls: `App.renderThrone()`, `App.renderTactical()`
- API calls go through `App.api(path, opts)` — never raw `fetch`

---

## Iconography

AURUM uses Unicode symbols instead of icon libraries. This keeps the build zero-dependency
and maintains the terminal aesthetic.

| Symbol | Usage                |
|--------|----------------------|
| `⚔`   | Logo / brand mark    |
| `▲`    | Wake / start event   |
| `▼`    | Sleep / stop event   |
| `✕`    | Failed / error       |
| `●`    | Started / online     |
| `■`    | Stopped / disabled   |
| `◷`    | Scheduled            |
| `⊘`    | Kill switch          |
| `↻`    | Refresh              |
| `+`    | Create / add         |

**Rule**: No emoji. No icon fonts. No SVG icon packs. Unicode only.

---

## The Command Palette

Triggered via `Ctrl+P`. This is the power-user's primary interface.

- Appears at 20% from top, centered, 500px wide
- Background: `--bg-1` with `--border-light` border
- Input: 15px sans, full width, transparent background
- Results: filterable list with icon, label, and keyboard shortcut hint
- Selected item: `--bg-3` background
- **Every action in the UI should be reachable from the command palette**

---

## Responsive Behavior

AURUM is designed for desktop monitors (1920px+ ideally) but degrades gracefully.

| Breakpoint | Behavior                                          |
|------------|---------------------------------------------------|
| > 1200px   | Full layout, all columns visible                  |
| 900–1200px | Tactical and Forge collapse to single column      |
| < 900px    | Status cards go to 2-column, all layouts stack    |

**Rule**: Never hide critical information on smaller screens. Reflow, don't remove.

---

## Voice & Tone

- **Labels**: Short, uppercase, military. "MARINES", not "Your Strategies"
- **Status text**: Terse and factual. "ONLINE", "OFFLINE", "3 active"
- **Errors**: Direct, no apologies. "Primarch unreachable" not "Oops, we can't connect"
- **Actions**: Imperative verbs. "Wake", "Disable", "Kill", "Launch"
- **No humor in the UI**. The theme is in the naming, not the copy.
- **Numbers first**. Show the value, then the label. "$1,234.56" before "Today's P&L"

---

## Dark Theme — No Light Mode

AURUM is dark-only. There is no light theme. There will never be a light theme.
Traders stare at screens for hours. Dark backgrounds reduce eye strain and make
colored indicators (green P&L, red losses, amber warnings) pop with maximum contrast.

The background progression (`--bg-0` → `--bg-1` → `--bg-2` → `--bg-3`) creates depth
through luminance layering, not shadows. Each step up is roughly +8 lightness in HSL.
