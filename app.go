package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ════════════════════════════════════════════════════════════════════════
// CONCURRENCY MODEL (Zero-Lock, Channel-Based)
//
//  ┌──────────────┐    chan FetchResult     ┌────────────────┐
//  │ Data Worker   │ ─────────────────────► │ UI Thread      │
//  │ (goroutine)   │                        │ (tview loop)   │
//  │               │                        │                │
//  │ HTTP calls    │    chan struct{}        │ state lives    │
//  │ RPC calls     │ ◄───────────────────── │ here ONLY      │
//  │ (blocking OK) │    "please refetch"    │ (no locks!)    │
//  └──────────────┘                        └────────────────┘
//
// RULE: State is ONLY read/written on the UI thread (inside QueueUpdateDraw).
//       The worker goroutine NEVER touches state directly.
//       Instead, it sends a FetchResult through a channel.
// ════════════════════════════════════════════════════════════════════════

// FetchResult is the payload sent from the worker goroutine to the UI thread.
type FetchResult struct {
	Matches        []Match
	SystemStats    SystemStats
	RequestedID    uint
	SelectedMatch  *Match
	MatchResults   []MatchResult
	VaultStats     *VaultStats
	// ChainStatuses maps Match.ID → on-chain status uint8 (0=Open,1=Locked,2=Resolved)
	// Populated for every match that has a VaultAddress.
	ChainStatuses  map[uint]uint8
	ChainConnected bool
	ChainError     error
	APIConnected   bool
}

// App is the main TUI application.
type App struct {
	tviewApp *tview.Application

	// Panels
	headerView  *tview.TextView
	matchTable  *tview.Table
	detailView  *tview.TextView
	statsView   *tview.TextView
	sourcesView *tview.TextView
	systemView  *tview.TextView
	logView     *tview.TextView
	inputField  *tview.InputField

	// Clients
	api          *APIClient   // Admin API (HTTP)
	chain        *ChainClient // On-chain (RPC)
	chainInitErr error        // Stored so Run() can display it in the TUI log

	// State — ONLY accessed from UI thread (no mutex needed)
	state     AppState
	startedAt time.Time

	// Channels
	fetchCh chan struct{} // Signal worker to refetch (buffered, size 1)
	stopCh  chan struct{} // Signal all goroutines to stop
}

// AppState holds all data required for rendering.
type AppState struct {
	Matches         []Match
	SelectedMatchID uint
	SelectedMatch   *Match
	MatchResults    []MatchResult
	VaultStats      *VaultStats
	ChainStatuses   map[uint]uint8 // on-chain status per match ID
	SystemStats     SystemStats
	ChainConnected  bool
	ChainError      error
	APIConnected    bool
}

type SystemStats struct {
	TotalMatches int
	Open         int
	Locked       int
	Finished     int
	TotalResults int
}

// NewApp creates and configures the TUI.
func NewApp(api *APIClient) *App {
	// Set global theme to Black
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorBlack
	tview.Styles.ContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorBlack
	tview.Styles.BorderColor = tcell.ColorDarkCyan
	tview.Styles.TitleColor = tcell.ColorAqua
	tview.Styles.GraphicsColor = tcell.ColorDarkCyan
	tview.Styles.PrimaryTextColor = tcell.ColorWhite
	tview.Styles.SecondaryTextColor = tcell.ColorYellow
	tview.Styles.TertiaryTextColor = tcell.ColorGreen
	tview.Styles.InverseTextColor = tcell.ColorBlack
	tview.Styles.ContrastSecondaryTextColor = tcell.ColorDarkCyan

	a := &App{
		tviewApp:  tview.NewApplication(),
		api:       api,
		fetchCh:   make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		startedAt: time.Now(),
	}

	// Build all panels
	a.headerView = a.buildHeader()
	a.matchTable = a.buildMatchTable()
	a.detailView = a.buildDetailView()
	a.statsView = a.buildStatsView()
	a.sourcesView = a.buildSourcesView()
	a.systemView = a.buildSystemView()
	a.logView = a.buildLogView()
	a.inputField = a.buildInput()

	// ── Layout ──────────────────────────────────────────────────────────
	centerColumn := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.detailView, 9, 0, false).
		AddItem(a.statsView, 12, 0, false).
		AddItem(a.sourcesView, 0, 1, false)

	content := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(a.matchTable, 0, 3, false).
		AddItem(centerColumn, 0, 4, false).
		AddItem(a.systemView, 0, 2, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.headerView, 1, 0, false).
		AddItem(content, 0, 1, false).
		AddItem(a.logView, 10, 0, false).
		AddItem(a.inputField, 3, 0, true)

	a.tviewApp.SetRoot(root, true).EnableMouse(true)
	a.tviewApp.SetFocus(a.inputField)

	// Global key bindings
	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			a.tviewApp.SetFocus(a.inputField)
			return nil
		case tcell.KeyTab:
			if a.inputField.HasFocus() {
				a.tviewApp.SetFocus(a.matchTable)
			} else {
				a.tviewApp.SetFocus(a.inputField)
			}
			return nil
		}
		return event
	})

	return a
}

// Run starts the TUI with live refresh.
func (a *App) Run() error {
	// Welcome messages
	a.addLog("[aqua::b]╔══════════════════════════════════════════════╗")
	a.addLog("[aqua::b]║  [green::b]EcoRound Simulator v2.0[aqua::b]                     ║")
	a.addLog("[aqua::b]║  [white]Chainlink CRE × Morpho Vault × Base[aqua::b]       ║")
	a.addLog("[aqua::b]╚══════════════════════════════════════════════╝")
	a.addLog("[darkgray]Type [yellow]help [darkgray]for commands  |  [yellow]Tab [darkgray]switch focus  |  [yellow]Esc [darkgray]command input")

	if a.chainInitErr != nil {
		a.addLog(fmt.Sprintf("[red]⚠ Chain init failed: %v", a.chainInitErr))
		a.addLog("[red]  match create requires chain — check RPC_URL / OWNER_PRIVATE_KEY in .env")
	} else if a.chain != nil {
		a.addLog("[green]✓ Chain connected")
	}

	// Start worker + UI ticker
	go a.dataWorker()
	go a.uiTicker()

	// Trigger initial fetch
	a.requestFetch()

	return a.tviewApp.Run()
}

// ════════════════════════════════════════════════════════════════════════
// WORKER GOROUTINE — runs in background, never touches UI or state
// ════════════════════════════════════════════════════════════════════════

func (a *App) dataWorker() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.fetchCh:
			a.doFetch()
		case <-ticker.C:
			a.doFetch()
		case <-a.stopCh:
			return
		}
	}
}

// doFetch runs blocking HTTP + RPC calls, then sends result to UI thread.
func (a *App) doFetch() {
	// 1. Read which match ID to fetch (safely via QueueUpdate)
	idCh := make(chan uint, 1)
	a.tviewApp.QueueUpdate(func() {
		idCh <- a.state.SelectedMatchID
	})
	requestedID := <-idCh

	// 2. Fetch all matches from Admin API
	matches, err := a.api.ListMatches("")
	apiConnected := err == nil
	if matches == nil {
		matches = []Match{}
	}

	// 3. Compute stats from the fetched matches
	var sysStats SystemStats
	sysStats.TotalMatches = len(matches)
	for _, m := range matches {
		switch m.Status {
		case "open":
			sysStats.Open++
		case "locked":
			sysStats.Locked++
		case "finished":
			sysStats.Finished++
		}
		sysStats.TotalResults += len(m.Results)
	}

	// 4. Fetch on-chain status for ALL matches (1 RPC call per vault — fast).
	chainStatuses := map[uint]uint8{}
	if a.chain != nil {
		for _, m := range matches {
			if m.VaultAddress != "" {
				if s, err := a.chain.ReadVaultStatus(m.VaultAddress); err == nil {
					chainStatuses[m.ID] = s
				}
			}
		}
	}

	// 5. Fetch full vault stats for selected match (deposits, yield, winner)
	var selectedMatch *Match
	var matchResults []MatchResult
	var vaultStats *VaultStats
	var chainConnected bool
	var chainError error

	if requestedID > 0 && apiConnected {
		m, err := a.api.GetMatch(requestedID)
		if err == nil && m != nil {
			selectedMatch = m
			matchResults = m.Results

			// On-chain vault stats (RPC call — may be slow)
			if a.chain != nil && m.VaultAddress != "" {
				chainConnected = true
				params, err := a.chain.ReadVaultStats(m.VaultAddress)
				if err == nil {
					vaultStats = params
				} else {
					chainError = err
				}
			}
		}
	}

	// 6. Send result to UI thread
	result := FetchResult{
		Matches:        matches,
		SystemStats:    sysStats,
		RequestedID:    requestedID,
		SelectedMatch:  selectedMatch,
		MatchResults:   matchResults,
		VaultStats:     vaultStats,
		ChainStatuses:  chainStatuses,
		ChainConnected: chainConnected,
		ChainError:     chainError,
		APIConnected:   apiConnected,
	}

	a.tviewApp.QueueUpdateDraw(func() {
		a.applyFetchResult(result)
		a.refreshAll()
	})
}

// applyFetchResult merges worker data into state.
func (a *App) applyFetchResult(r FetchResult) {
	a.state.Matches = r.Matches
	a.state.SystemStats = r.SystemStats
	a.state.APIConnected = r.APIConnected
	// ChainStatuses covers all matches — always safe to update
	a.state.ChainStatuses = r.ChainStatuses

	// Only update details if selection hasn't changed during fetch
	if a.state.SelectedMatchID == r.RequestedID {
		a.state.SelectedMatch = r.SelectedMatch
		a.state.MatchResults = r.MatchResults
		a.state.VaultStats = r.VaultStats
		a.state.ChainConnected = r.ChainConnected
		a.state.ChainError = r.ChainError
	}

	// Auto-select first match on initial load
	if a.state.SelectedMatchID == 0 && len(r.Matches) > 0 {
		a.state.SelectedMatchID = r.Matches[0].ID
	}
}

// ════════════════════════════════════════════════════════════════════════
// UI TICKER
// ════════════════════════════════════════════════════════════════════════

func (a *App) uiTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.tviewApp.QueueUpdateDraw(func() {
				a.refreshAll()
			})
		case <-a.stopCh:
			return
		}
	}
}

// ════════════════════════════════════════════════════════════════════════
// STATE MUTATIONS — called from UI thread only
// ════════════════════════════════════════════════════════════════════════

func (a *App) selectMatch(id uint) {
	a.state.SelectedMatchID = id
	a.state.SelectedMatch = nil
	a.refreshAll()
	a.requestFetch()
}

func (a *App) requestFetch() {
	select {
	case a.fetchCh <- struct{}{}:
	default:
	}
}

func (a *App) refreshAll() {
	a.refreshHeader()
	a.refreshMatchTable()
	a.refreshDetailView()
	a.refreshStatsView()
	a.refreshSourcesView()
	a.refreshSystemView()
}

func (a *App) GetSelectedMatchID() uint {
	return a.state.SelectedMatchID
}
