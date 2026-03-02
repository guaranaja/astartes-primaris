# AURUM — Interface Layer

> AAA quality interfaces for commanding the Imperium.

## Tech

- **Web**: Next.js 14 + TypeScript + Tailwind CSS
- **Real-time**: WebSocket subscriptions via Vox bridge
- **Charts**: TradingView Lightweight Charts + custom D3 visualizations
- **State**: React Query (server state) + Zustand (client state)
- **Theme**: Dark mode first, military-precision aesthetic

## Views

### Command Throne (Home)
Full Imperium overview — all Fortresses, Companies, Marines at a glance.
Real-time P&L, active positions, system health.

### Tactical Display
Real-time strategy execution view. Watch Marines wake, decide, and act.
Live order flow, signal visualization, position changes.

### Forge Console
Submit backtest and optimization jobs. View results with interactive
heatmaps, equity curves, and statistical analysis.

### Librarium Browser
Data exploration and charting. Query historical data, inspect tick data,
verify data quality.

### Codex Editor
Configuration management UI. Edit strategy parameters, risk limits,
scheduling rules. Preview changes before applying.

### War Room
Multi-monitor layout with customizable panels. Combine any views
into a single command-center display.

## Design Principles

- Sub-100ms interaction response
- Real-time data via WebSocket (no polling)
- Keyboard-first navigation (Vim-style shortcuts)
- Information density like Bloomberg, aesthetics like Linear

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 3000  | HTTP     | Web application      |
