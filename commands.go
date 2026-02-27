package main

import (
	"fmt"
	"strconv"
	"strings"
)

// executeCommand parses and runs a user command.
func (a *App) executeCommand(input string) {
	parts := parseArgs(input)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "match", "m":
		a.handleMatch(args)
	case "help", "h":
		a.showHelp()
	case "quit", "exit", "q":
		a.tviewApp.Stop()
	case "clear", "cls":
		a.logView.Clear()
		a.addLog("Log cleared")
	default:
		a.addLog(fmt.Sprintf("[red]Unknown command: %s[white] — type [yellow]help", cmd))
	}
}

func (a *App) handleMatch(args []string) {
	if len(args) == 0 {
		a.addLog("[red]Usage: match <create|list|select|status|simulate>")
		return
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "create", "c":
		a.cmdMatchCreate(args[1:])
	case "list", "ls":
		a.cmdMatchList(args[1:])
	case "select", "sel", "s":
		a.cmdMatchSelect(args[1:])
	case "status", "st":
		a.cmdMatchStatus(args[1:])
	case "simulate", "sim":
		a.cmdMatchSimulate(args[1:])
	default:
		a.addLog(fmt.Sprintf("[red]Unknown: match %s", sub))
	}
}

// match create <teamA> <teamB> [bestOf] [event...]
func (a *App) cmdMatchCreate(args []string) {
	if len(args) < 2 {
		a.addLog("[red]Usage: match create <teamA> <teamB> [bestOf] [event...]")
		return
	}

	teamA := args[0]
	teamB := args[1]
	bestOf := 3
	event := ""

	if len(args) >= 3 {
		if bo, err := strconv.Atoi(args[2]); err == nil {
			bestOf = bo
			if len(args) >= 4 {
				event = strings.Join(args[3:], " ")
			}
		} else {
			event = strings.Join(args[2:], " ")
		}
	}

	a.addLog(fmt.Sprintf("[yellow]Creating match[white]: [aqua]%s [darkgray]vs [orangered]%s ...", teamA, teamB))

	// Run in goroutine to not block TUI
	go func() {
		req := CreateMatchRequest{
			TeamAName: teamA,
			TeamATag:  toTag(teamA),
			TeamBName: teamB,
			TeamBTag:  toTag(teamB),
			BestOf:    bestOf,
			Event:     event,
		}

		// Step 1: create match record in DB
		match, err := a.api.CreateMatch(req)
		if err != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.addLog(fmt.Sprintf("[red]API Error: %v", err))
			})
			return
		}

		a.tviewApp.QueueUpdateDraw(func() {
			a.addLog(fmt.Sprintf("[green]Match #%d created[white]: [aqua]%s [darkgray]vs [orangered]%s",
				match.ID, teamA, teamB))
		})

		// Step 2: deploy vault on-chain
		if a.chain == nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.addLog("[yellow]Chain not configured — vault not deployed on-chain")
				a.selectMatch(match.ID)
			})
			return
		}

		a.tviewApp.QueueUpdateDraw(func() {
			a.addLog("[yellow]Deploying vault on-chain...")
		})

		onChainID, vaultAddr, chainErr := a.chain.CreateMatchOnChain(teamA, teamB)
		if chainErr != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.addLog(fmt.Sprintf("[red]Chain Error: %v", chainErr))
				a.selectMatch(match.ID)
			})
			return
		}

		// Step 3: save vault address back to DB
		if updateErr := a.api.UpdateMatchVault(match.ID, onChainID, vaultAddr.Hex()); updateErr != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.addLog(fmt.Sprintf("[red]Vault update Error: %v", updateErr))
				a.selectMatch(match.ID)
			})
			return
		}

		a.tviewApp.QueueUpdateDraw(func() {
			a.addLog(fmt.Sprintf("[green]Vault deployed[white]: on-chain #%d → [aqua]%s", onChainID, vaultAddr.Hex()))
			a.selectMatch(match.ID)
			a.requestFetch()
		})
	}()
}

// match list [status]
func (a *App) cmdMatchList(args []string) {
	a.requestFetch()
	if len(args) > 0 {
		a.addLog(fmt.Sprintf("Filtered by: [yellow]%s", args[0]))
	} else {
		a.addLog("Match list refreshed")
	}
}

// match select <id>
func (a *App) cmdMatchSelect(args []string) {
	if len(args) < 1 {
		a.addLog("[red]Usage: match select <id>")
		return
	}
	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		a.addLog("[red]Invalid match ID")
		return
	}
	a.selectMatch(uint(id))
}

// match status <open|locked|finished|cancelled>
func (a *App) cmdMatchStatus(args []string) {
	selectedID := a.GetSelectedMatchID()
	if selectedID == 0 {
		a.addLog("[red]No match selected. Use [yellow]match select <id>")
		return
	}
	if len(args) < 1 {
		a.addLog("[red]Usage: match status <open|locked|finished|cancelled>")
		return
	}

	status := strings.ToLower(args[0])
	valid := map[string]bool{"open": true, "locked": true, "finished": true, "cancelled": true}
	if !valid[status] {
		a.addLog("[red]Invalid status. Use: open, locked, finished, cancelled")
		return
	}

	a.addLog(fmt.Sprintf("[yellow]Updating status[white] → [yellow]%s ...", strings.ToUpper(status)))

	// Run API call in goroutine
	go func() {
		err := a.api.UpdateMatchStatus(selectedID, status)

		a.tviewApp.QueueUpdateDraw(func() {
			if err != nil {
				a.addLog(fmt.Sprintf("[red]API Error: %v", err))
				return
			}
			a.addLog(fmt.Sprintf("[green]Match #%d[white] status → [yellow]%s", selectedID, strings.ToUpper(status)))
			a.requestFetch()
		})
	}()
}

// match simulate <source> <match_status> [scoreA] [scoreB]
func (a *App) cmdMatchSimulate(args []string) {
	selectedID := a.GetSelectedMatchID()
	if selectedID == 0 {
		a.addLog("[red]No match selected. Use [yellow]match select <id>")
		return
	}
	if len(args) < 2 {
		a.addLog("[red]Usage: match simulate <source> <started|ended> [scoreA scoreB]")
		return
	}

	source := strings.ToLower(args[0])
	validSrc := map[string]bool{"pandascore": true, "vlr": true, "liquipedia": true}
	if !validSrc[source] {
		a.addLog("[red]Invalid source. Use: pandascore, vlr, liquipedia")
		return
	}

	matchStatus := strings.ToLower(args[1])
	validStatus := map[string]bool{"upcoming": true, "started": true, "ended": true}
	if !validStatus[matchStatus] {
		a.addLog("[red]Invalid status. Use: upcoming, started, ended")
		return
	}

	scoreA, scoreB := 0, 0
	winner := ""

	if matchStatus == "ended" {
		if len(args) < 4 {
			a.addLog("[red]Usage for ended: match simulate <source> ended <scoreA> <scoreB>")
			return
		}
		var e1, e2 error
		scoreA, e1 = strconv.Atoi(args[2])
		scoreB, e2 = strconv.Atoi(args[3])
		if e1 != nil || e2 != nil {
			a.addLog("[red]Scores must be numbers")
			return
		}
		winner = "TeamA"
		if scoreB > scoreA {
			winner = "TeamB"
		}
	}

	a.addLog(fmt.Sprintf("[yellow]Submitting result[white] %s: %s ...", source, matchStatus))

	// Run API call in goroutine
	go func() {
		err := a.api.SetResult(selectedID, SetResultRequest{
			Source:      source,
			MatchStatus: matchStatus,
			Winner:      winner,
			ScoreA:      scoreA,
			ScoreB:      scoreB,
			MapCount:    scoreA + scoreB,
		})

		a.tviewApp.QueueUpdateDraw(func() {
			if err != nil {
				a.addLog(fmt.Sprintf("[red]API Error: %v", err))
				return
			}
			if matchStatus == "ended" {
				a.addLog(fmt.Sprintf("[green]%s[white]: [yellow]%s [darkgray]— %s (%d-%d)", source, matchStatus, winner, scoreA, scoreB))
			} else {
				a.addLog(fmt.Sprintf("[green]%s[white]: [yellow]%s", source, matchStatus))
			}
			a.requestFetch()
		})
	}()
}

func (a *App) showHelp() {
	a.addLog("[yellow]──── Commands ────")
	a.addLog("[aqua]match create   [white]<teamA> <teamB> [bestOf] [event]")
	a.addLog("[aqua]match list     [white][status]")
	a.addLog("[aqua]match select   [white]<id>       [darkgray]or click a row")
	a.addLog("[aqua]match status   [white]<open|locked|finished|cancelled>")
	a.addLog("[aqua]match simulate [white]<source> <started|ended> [scoreA scoreB]")
	a.addLog("[aqua]clear          [white]Clear log")
	a.addLog("[aqua]quit           [white]Exit")
	a.addLog("[yellow]──── Shortcuts ────")
	a.addLog("[aqua]m c  [white]= match create   [aqua]m sim [white]= match simulate")
	a.addLog("[aqua]m ls [white]= match list     [aqua]m st  [white]= match status")
}

// ── Helpers ─────────────────────────────────────────────────────────────

func toTag(name string) string {
	if len(name) <= 4 {
		return strings.ToUpper(name)
	}
	return strings.ToUpper(name[:3])
}

func parseArgs(input string) []string {
	var args []string
	var cur strings.Builder
	inQ := false
	qCh := rune(0)

	for _, ch := range input {
		if inQ {
			if ch == qCh {
				inQ = false
			} else {
				cur.WriteRune(ch)
			}
		} else if ch == '"' || ch == '\'' {
			inQ = true
			qCh = ch
		} else if ch == ' ' {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}
