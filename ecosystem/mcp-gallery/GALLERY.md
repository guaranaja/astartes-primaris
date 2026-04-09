# Astartes MCP Gallery

MCP (Model Context Protocol) servers available to the Astartes ecosystem.
Once ONNX models are mature, MCP servers become the next integration layer —
strategies can query financial context or execute across platforms.

## Registry

| MCP Server | Category | Capabilities | Status | Config |
|------------|----------|-------------|--------|--------|
| monarch-money | Finance | Accounts, transactions, budgets, goals, net worth | Available | configs/monarch-money.json |
| alpaca | Trading | Stocks, options, crypto, market data, orders | Available | configs/alpaca.json |
| github | DevOps | Repos, PRs, issues, actions | Active | configs/github.json |
| firefly-iii | Finance | Accounts, transactions, budgets, bills | Planned | — |
| ibkr | Trading | TWS API, orders, market data | Planned | — |
| tastytrade | Trading | Options chains, orders, positions | Planned | — |

## Adding an MCP Server

1. Verify the MCP server is available and documented
2. Create a config JSON in `configs/` with connection details
3. Add an entry to this GALLERY
4. Tag with `# ECOSYSTEM: mcp-gallery`
