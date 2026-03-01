# EcoRound — Admin Panel (panel-v2)

Go TUI (terminal UI) admin panel for managing EcoRound matches. Built with [tview](https://github.com/rivo/tview).

Connects to both the **Admin HTTP API** (api-simulator) and the **Base chain** (Tenderly fork via go-ethereum RPC) to deploy vaults on-chain and manage match state in real time.

## Screenshot

```
┌ MATCHES ──────────────────┐  ┌ MATCH DETAIL ─────────────────┐  ┌ SYSTEM ──────────────┐
│ ID  Team A  Team B  Status│  │ DRX vs RRQ                     │  │ RPC  ● Connected      │
│ ►7  DRX  vs RRQ    OPEN  │  │ Status      OPEN               │  │ Chain  Base (Tenderly)│
└───────────────────────────┘  │ DB ID       #7                 │  │ Chain ID  84531       │
                               │ On-Chain ID #3                 │  │ API  ● Connected      │
┌ BETTING STATS ─────────────────────────────────────────────────┐ └──────────────────────┘
│ Data Source ● on-chain    Total Pool  $0.00 USDC               │
│ DRX 0.0%  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   $0.00     │
│ RRQ 0.0%  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   $0.00     │
│ Vault Yield  $0.000000 USDC                                    │
└────────────────────────────────────────────────────────────────┘
```

## Commands

| Command | Description |
|---|---|
| `match create <teamA> <teamB> [bestOf] [event]` | Deploy vault on-chain + save to DB |
| `match list` | Refresh match list |
| `match select <id>` | Select a match to view details |
| `match status <open\|locked\|finished\|cancelled>` | Update match status in DB |
| `match simulate <source> <started\|ended> [scoreA scoreB]` | Set a source result (simulates data source reporting) |
| `clear` | Clear event log |
| `help` | Show all commands |
| `quit` | Exit |

**Shortcuts**: `m c` = match create, `m ls` = match list, `m st` = match status, `m sim` = match simulate

## Match Create Flow

`match create DRX RRQ` does three things automatically:

1. **Deploy vault on-chain** — calls `FactoryMatch.createMatch("DRX", "RRQ")`, waits for tx receipt, reads `onChainMatchId` and `vaultAddress`
2. **Save to DB** — calls Admin API with on-chain ID and vault address already set (single source of truth — no ID mismatch)
3. **Display** — logs vault address and on-chain ID in the event log, auto-selects the new match

## Setup

```bash
cd panel-v2

# Configure environment
# Copy and edit .env (must be in same directory as binary or cwd)
#   DATABASE_URL=postgresql://...   (same Neon DB as api-simulator)
#   RPC_URL=https://virtual.rpc.tenderly.co/...
#   FACTORY_ADDRESS=0x602473fc59ff5eefbe5d6c86d3af5c64ac7987bc
#   OWNER_PRIVATE_KEY=0x...
#   API_URL=http://localhost:8080

# Build
go build -o panel-v2.exe .

# Run (from the panel-v2 directory)
./panel-v2.exe
# or: go run .
```

## Environment Variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | Neon PostgreSQL connection string |
| `RPC_URL` | Tenderly fork RPC URL |
| `FACTORY_ADDRESS` | Deployed FactoryMatch contract address |
| `OWNER_PRIVATE_KEY` | Owner wallet private key (0x-prefixed) |
| `API_URL` | Admin API base URL (default: `http://localhost:8080`) |

> The `.env` is loaded from the current working directory first, then from the directory containing the binary.

## Architecture

```
panel-v2/
├── main.go        # Entry point — loads .env, initializes clients, launches TUI
├── app.go         # TUI app, channel-based worker goroutine, state management
├── chain.go       # ChainClient — go-ethereum RPC: createMatch, readVaultStats
├── api_client.go  # APIClient — HTTP calls to api-simulator admin API
├── commands.go    # Command parser and handlers (match create/list/simulate)
├── views.go       # Panel renderers (match table, detail, stats, sources, system)
└── models.go      # Shared data models (Match, MatchResult, VaultStats)
```

## Concurrency Model

All state lives on the UI thread. A background worker goroutine handles blocking HTTP + RPC calls and sends results via channels — no mutexes needed.

- `fetchCh chan struct{}` — signals the worker to refetch
- Data worker polls every **15 seconds** + on-demand after any command
- All UI updates go through `tviewApp.QueueUpdateDraw()`

## Simulate a Full Match Flow

```
# 1. Create match (deploys vault on-chain)
> match create Sentinels NRG

# 2. Simulate sources reporting "started" (triggers CRE → lockMatch)
> match simulate pandascore started
> match simulate vlr started
> match simulate liquipedia started

# 3. CRE oracle fires → calls lockMatch() → USDC enters Morpho

# 4. Simulate sources reporting "ended" with winner
> match simulate pandascore ended 2 0
> match simulate vlr ended 2 0
> match simulate liquipedia ended 2 0

# 5. CRE oracle fires → calls resolveMatch(TeamA) → yield distributed

# 6. Users claim on the frontend
```
