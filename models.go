package main

import "time"

// Match represents a match returned by the Admin API.
// No GORM tags — this is a pure JSON model now.
type Match struct {
	ID             uint      `json:"id"`
	OnChainMatchID uint      `json:"on_chain_match_id"`
	VaultAddress   string    `json:"vault_address"`
	TeamAName      string    `json:"team_a_name"`
	TeamATag       string    `json:"team_a_tag"`
	TeamBName      string    `json:"team_b_name"`
	TeamBTag       string    `json:"team_b_tag"`
	Status         string    `json:"status"`
	BestOf         int       `json:"best_of"`
	Event          string    `json:"event"`
	StartTime      time.Time `json:"start_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Results []MatchResult `json:"results,omitempty"`
}

// MatchResult represents a result reported by a data source.
type MatchResult struct {
	ID          uint      `json:"id"`
	MatchID     uint      `json:"match_id"`
	Source      string    `json:"source"`
	MatchStatus string    `json:"match_status"`
	Winner      string    `json:"winner"`
	ScoreA      int       `json:"score_a"`
	ScoreB      int       `json:"score_b"`
	MapCount    int       `json:"map_count"`
	ReportedAt  time.Time `json:"reported_at"`
}
