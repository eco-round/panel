package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ── Match List Table ────────────────────────────────────────────────────

func (a *App) buildMatchTable() *tview.Table {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	table.SetBorder(true).SetTitle(" MATCHES ").SetTitleAlign(tview.AlignLeft)
	table.SetBorderColor(tcell.ColorDarkCyan)
	table.SetTitleColor(tcell.ColorAqua)

	table.SetSelectedFunc(func(row, col int) {
		if row == 0 {
			return
		}
		idCell := table.GetCell(row, 0)
		if idCell != nil {
			var id uint
			fmt.Sscanf(strings.TrimSpace(idCell.Text), "%d", &id)
			a.selectMatch(id)
		}
	})

	return table
}

func (a *App) refreshMatchTable() {
	matches := a.state.Matches

	table := a.matchTable
	table.Clear()

	headers := []string{"ID", "Team A", " ", "Team B", "Status", "Event"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		if i == 2 {
			cell.SetExpansion(0)
		}
		table.SetCell(0, i, cell)
	}

	for i, m := range matches {
		row := i + 1
		prefix := " "
		idColor := tcell.ColorWhite
		if a.state.SelectedMatchID == m.ID {
			prefix = "►"
			idColor = tcell.ColorYellow
		}

		statusColor := statusToColor(m.Status)

		table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%s%d", prefix, m.ID)).SetTextColor(idColor).SetExpansion(1))
		table.SetCell(row, 1, tview.NewTableCell(m.TeamATag).SetTextColor(tcell.ColorAqua).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell("vs").SetTextColor(tcell.ColorDarkGray).SetExpansion(0))
		table.SetCell(row, 3, tview.NewTableCell(m.TeamBTag).SetTextColor(tcell.ColorOrangeRed).SetExpansion(1))
		table.SetCell(row, 4, tview.NewTableCell(strings.ToUpper(m.Status)).SetTextColor(statusColor).SetExpansion(1))
		table.SetCell(row, 5, tview.NewTableCell(truncStr(m.Event, 22)).SetTextColor(tcell.ColorDarkGray).SetExpansion(1))
	}

	table.SetTitle(fmt.Sprintf(" MATCHES (%d) ", len(matches)))
}

// ── Match Detail Panel ──────────────────────────────────────────────────

func (a *App) buildDetailView() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" MATCH DETAIL ").SetTitleAlign(tview.AlignLeft)
	tv.SetBorderColor(tcell.ColorDarkCyan)
	tv.SetTitleColor(tcell.ColorAqua)
	return tv
}

func (a *App) refreshDetailView() {
	a.detailView.Clear()
	if a.state.SelectedMatchID == 0 {
		fmt.Fprint(a.detailView, "[darkgray]  No match selected.\n  Use [yellow]match select <id>")
		return
	}

	m := a.state.SelectedMatch
	if m == nil {
		fmt.Fprint(a.detailView, "[red]  Match not found or loading...")
		return
	}

	sc := statusColorName(m.Status)
	fmt.Fprintf(a.detailView, "  [aqua::b]%s[white] vs [orangered::b]%s\n", m.TeamAName, m.TeamBName)
	fmt.Fprintf(a.detailView, "  [white]Status      [%s]%s\n", sc, strings.ToUpper(m.Status))
	fmt.Fprintf(a.detailView, "  [white]Event       [darkgray]%s\n", m.Event)
	fmt.Fprintf(a.detailView, "  [white]Best Of     [darkgray]%d\n", m.BestOf)
	if !m.StartTime.IsZero() {
		fmt.Fprintf(a.detailView, "  [white]Start       [darkgray]%s\n", m.StartTime.Format("2006-01-02 15:04"))
	}
	fmt.Fprintf(a.detailView, "  [white]DB ID       [yellow]#%d\n", m.ID)
	if m.OnChainMatchID > 0 {
		fmt.Fprintf(a.detailView, "  [white]On-Chain ID [yellow]#%d\n", m.OnChainMatchID)
	}
	if m.VaultAddress != "" {
		short := m.VaultAddress
		if len(short) > 10 {
			short = short[:6] + "…" + short[len(short)-4:]
		}
		fmt.Fprintf(a.detailView, "  [white]Vault       [darkgray]%s", short)
	}
}

// ── Betting Stats Panel (with bars) ─────────────────────────────────────

func (a *App) buildStatsView() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" BETTING STATS ").SetTitleAlign(tview.AlignLeft)
	tv.SetBorderColor(tcell.ColorDarkCyan)
	tv.SetTitleColor(tcell.ColorAqua)
	return tv
}

func (a *App) refreshStatsView() {
	a.statsView.Clear()
	if a.state.SelectedMatchID == 0 {
		fmt.Fprint(a.statsView, "[darkgray]  Select a match to see stats")
		return
	}

	m := a.state.SelectedMatch
	if m == nil {
		return
	}
	tagA, tagB := m.TeamATag, m.TeamBTag

	var totalA, totalB, totalDeposits, yieldBalance float64
	chainStatus := "[darkgray]offline"

	if a.state.ChainConnected && m.VaultAddress != "" {
		if a.state.VaultStats != nil {
			// USDC has 6 decimals
			totalA = float64(a.state.VaultStats.TotalTeamA.Uint64()) / 1e6
			totalB = float64(a.state.VaultStats.TotalTeamB.Uint64()) / 1e6
			totalDeposits = float64(a.state.VaultStats.TotalDeposits.Uint64()) / 1e6
			yieldBalance = float64(a.state.VaultStats.YieldBalance.Uint64()) / 1e6
			chainStatus = "[green]● on-chain"
		} else if a.state.ChainError != nil {
			chainStatus = fmt.Sprintf("[red]● error: %v", a.state.ChainError)
		} else {
			chainStatus = "[yellow]● loading..."
		}
	} else {
		chainStatus = "[yellow]no vault"
	}

	total := totalA + totalB
	pctA, pctB := 0.0, 0.0
	if total > 0 {
		pctA = totalA / total * 100
		pctB = totalB / total * 100
	}

	barW := 40
	fillA := int(pctA / 100 * float64(barW))
	fillB := int(pctB / 100 * float64(barW))
	barA := "[aqua]" + strings.Repeat("█", fillA) + "[darkgray]" + strings.Repeat("░", barW-fillA)
	barB := "[orangered]" + strings.Repeat("█", fillB) + "[darkgray]" + strings.Repeat("░", barW-fillB)

	fmt.Fprintf(a.statsView, "  [white]Data Source  %s\n", chainStatus)
	fmt.Fprintf(a.statsView, "  [white]Total Pool   [green::b]$%.2f USDC\n\n", totalDeposits)
	fmt.Fprintf(a.statsView, "  [aqua::b]%s [darkgray]%.1f%%\n", tagA, pctA)
	fmt.Fprintf(a.statsView, "  %s [white]$%.2f\n\n", barA, totalA)
	fmt.Fprintf(a.statsView, "  [orangered::b]%s [darkgray]%.1f%%\n", tagB, pctB)
	fmt.Fprintf(a.statsView, "  %s [white]$%.2f\n\n", barB, totalB)
	fmt.Fprintf(a.statsView, "  [white]Vault Yield  [green]$%.6f USDC", yieldBalance)
}

// ── Sources Panel ───────────────────────────────────────────────────────

func (a *App) buildSourcesView() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" SOURCES ").SetTitleAlign(tview.AlignLeft)
	tv.SetBorderColor(tcell.ColorDarkCyan)
	tv.SetTitleColor(tcell.ColorAqua)
	return tv
}

func (a *App) refreshSourcesView() {
	a.sourcesView.Clear()
	if a.state.SelectedMatchID == 0 {
		fmt.Fprint(a.sourcesView, "[darkgray]  Select a match")
		return
	}

	sources := []string{"pandascore", "vlr", "liquipedia"}
	startedCount := 0
	endedCount := 0
	winnerVotes := map[string]int{}

	// Convert results slab to map for easy lookup
	resultsMap := make(map[string]MatchResult)
	for _, r := range a.state.MatchResults {
		resultsMap[r.Source] = r
	}

	for _, src := range sources {
		result, exists := resultsMap[src]
		if !exists {
			fmt.Fprintf(a.sourcesView, "  [darkgray]%-12s — pending —\n", src)
		} else {
			statusIcon := "[yellow]⏳"
			statusLabel := result.MatchStatus
			if result.MatchStatus == "started" {
				statusIcon = "[green]▶"
				startedCount++
			} else if result.MatchStatus == "ended" {
				statusIcon = "[green]✓"
				endedCount++
				winnerVotes[result.Winner]++
			}
			if result.Winner != "" {
				fmt.Fprintf(a.sourcesView, "  %s [white]%-12s [yellow]%s [darkgray](%d-%d)\n", statusIcon, src, result.Winner, result.ScoreA, result.ScoreB)
			} else {
				fmt.Fprintf(a.sourcesView, "  %s [white]%-12s [darkgray]%s\n", statusIcon, src, statusLabel)
			}
		}
	}

	// Consensus logic
	fmt.Fprintf(a.sourcesView, "\n  [aqua::b]── Consensus ──\n")

	// Started consensus (for lockMatch)
	if startedCount >= 2 {
		fmt.Fprintf(a.sourcesView, "  [white]Started   [green::b]%d/3 → LOCK ✓\n", startedCount)
	} else if startedCount > 0 {
		fmt.Fprintf(a.sourcesView, "  [white]Started   [yellow]%d/3\n", startedCount)
	}

	// Ended consensus (for resolveMatch)
	if endedCount >= 2 {
		// Find majority winner
		bestWinner := ""
		bestCount := 0
		for w, c := range winnerVotes {
			if c > bestCount {
				bestWinner = w
				bestCount = c
			}
		}
		if bestCount >= 2 {
			fmt.Fprintf(a.sourcesView, "  [white]Ended     [green::b]%d/3 → %s ✓", endedCount, bestWinner)
		} else {
			fmt.Fprintf(a.sourcesView, "  [white]Ended     [yellow]%d/3 — no consensus", endedCount)
		}
	} else if endedCount > 0 {
		fmt.Fprintf(a.sourcesView, "  [white]Ended     [yellow]%d/3", endedCount)
	} else if startedCount == 0 {
		fmt.Fprintf(a.sourcesView, "  [darkgray]  no data")
	}
}

// ── System Panel ────────────────────────────────────────────────────────

func (a *App) buildSystemView() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" SYSTEM ").SetTitleAlign(tview.AlignLeft)
	tv.SetBorderColor(tcell.ColorDarkCyan)
	tv.SetTitleColor(tcell.ColorAqua)
	return tv
}

func (a *App) refreshSystemView() {
	a.systemView.Clear()

	uptime := time.Since(a.startedAt).Truncate(time.Second)

	// Chain status
	chainIcon := "[red]● Disconnected"
	chainID := "—"
	if a.chain != nil {
		chainIcon = "[green]● Connected"
		chainID = a.chain.ChainID.String()
	}

	fmt.Fprint(a.systemView, "  [aqua::b]── Network ──\n")
	fmt.Fprintf(a.systemView, "  [white]RPC         %s\n", chainIcon)
	fmt.Fprint(a.systemView, "  [white]Chain       [yellow]Base (Tenderly)\n")
	fmt.Fprintf(a.systemView, "  [white]Chain ID    [darkgray]%s\n", chainID)
	fmt.Fprint(a.systemView, "  [white]Token       [green]USDC\n")
	fmt.Fprint(a.systemView, "  [white]Vault       [darkgray]Morpho ERC4626\n\n")

	fmt.Fprint(a.systemView, "  [aqua::b]── Contracts ──\n")
	fmt.Fprint(a.systemView, "  [white]Factory     [darkgray]0x6024…87bc\n")
	fmt.Fprint(a.systemView, "  [white]USDC        [darkgray]0x8335…fCD6\n")
	fmt.Fprint(a.systemView, "  [white]Morpho      [darkgray]0x050c…56f0\n\n")

	// API status
	apiIcon := "[red]● Offline"
	if a.state.APIConnected {
		apiIcon = "[green]● Connected"
	}

	fmt.Fprint(a.systemView, "  [aqua::b]── Database ──\n")
	fmt.Fprintf(a.systemView, "  [white]API         %s\n", apiIcon)
	fmt.Fprintf(a.systemView, "  [white]Provider    [darkgray]Neon PostgreSQL\n")
	fmt.Fprintf(a.systemView, "  [white]Matches     [yellow]%d\n", a.state.SystemStats.TotalMatches)
	fmt.Fprintf(a.systemView, "  [white]  Open      [darkcyan]%d\n", a.state.SystemStats.Open)
	fmt.Fprintf(a.systemView, "  [white]  Locked    [yellow]%d\n", a.state.SystemStats.Locked)
	fmt.Fprintf(a.systemView, "  [white]  Finished  [gray]%d\n", a.state.SystemStats.Finished)
	fmt.Fprintf(a.systemView, "  [white]Results     [darkgray]%d\n\n", a.state.SystemStats.TotalResults)

	fmt.Fprint(a.systemView, "  [aqua::b]── Session ──\n")
	fmt.Fprintf(a.systemView, "  [white]Uptime      [darkgray]%s\n", uptime)
	fmt.Fprintf(a.systemView, "  [white]Refresh     [darkgray]3s\n")
	fmt.Fprintf(a.systemView, "  [white]API         [green]● :8080\n")
}

// ── Event Log ───────────────────────────────────────────────────────────

func (a *App) buildLogView() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetWordWrap(true).
		SetScrollable(true).
		SetMaxLines(500)
	tv.SetBorder(true).SetTitle(" EVENT LOG ").SetTitleAlign(tview.AlignLeft)
	tv.SetBorderColor(tcell.ColorDarkCyan)
	tv.SetTitleColor(tcell.ColorAqua)
	return tv
}

func (a *App) addLog(msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(a.logView, " [darkgray][%s] [white]%s\n", ts, msg)
	a.logView.ScrollToEnd()
}

// ── Command Input ───────────────────────────────────────────────────────

func (a *App) buildInput() *tview.InputField {
	input := tview.NewInputField().
		SetLabel("  [aqua::b]> ").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite).
		SetLabelColor(tcell.ColorAqua)

	input.SetBorder(true).SetBorderColor(tcell.ColorDarkCyan)
	input.SetTitle(" COMMAND ").SetTitleAlign(tview.AlignLeft).SetTitleColor(tcell.ColorAqua)

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := input.GetText()
			if text != "" {
				a.executeCommand(text)
				input.SetText("")
			}
		}
	})

	return input
}

// ── Header Bar ──────────────────────────────────────────────────────────

func (a *App) buildHeader() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(tcell.NewRGBColor(30, 40, 55))
	return tv
}

func (a *App) refreshHeader() {
	a.headerView.Clear()
	right := "[darkgray]SIM: IDLE"
	if a.state.SelectedMatchID > 0 {
		m := a.state.SelectedMatch
		if m != nil {
			right = fmt.Sprintf("[yellow]Match #%d[white]: [aqua]%s [darkgray]vs [orangered]%s [white]| [%s]%s",
				m.ID, m.TeamATag, m.TeamBTag, statusColorName(m.Status), strings.ToUpper(m.Status))
		}
	}
	fmt.Fprintf(a.headerView, " [aqua::b]ECOROUND SIMULATOR                                                    %s ", right)
}

// ── Helpers ─────────────────────────────────────────────────────────────

func statusToColor(s string) tcell.Color {
	switch s {
	case "open":
		return tcell.ColorDarkCyan
	case "locked":
		return tcell.ColorYellow
	case "finished":
		return tcell.ColorGray
	case "cancelled":
		return tcell.ColorRed
	}
	return tcell.ColorWhite
}

func statusColorName(s string) string {
	switch s {
	case "open":
		return "darkcyan"
	case "locked":
		return "yellow"
	case "finished":
		return "gray"
	case "cancelled":
		return "red"
	}
	return "white"
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
