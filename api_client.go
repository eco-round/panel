package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient is an HTTP client for the api-simulator Admin API.
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client pointed at the given base URL.
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ── Response Types ──────────────────────────────────────────────────────

type ListMatchesResponse struct {
	Matches []Match `json:"matches"`
}

// ── Read Operations ─────────────────────────────────────────────────────

// ListMatches fetches all matches (optionally filtered by status).
func (c *APIClient) ListMatches(status string) ([]Match, error) {
	url := c.BaseURL + "/api/v1/admin/matches"
	if status != "" {
		url += "?status=" + status
	}

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET matches: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET matches: status %d: %s", resp.StatusCode, string(body))
	}

	var result ListMatchesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode matches: %w", err)
	}

	return result.Matches, nil
}

// GetMatch fetches a single match by ID (with results preloaded).
func (c *APIClient) GetMatch(id uint) (*Match, error) {
	url := fmt.Sprintf("%s/api/v1/admin/matches/%d", c.BaseURL, id)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET match %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET match %d: status %d: %s", id, resp.StatusCode, string(body))
	}

	var match Match
	if err := json.NewDecoder(resp.Body).Decode(&match); err != nil {
		return nil, fmt.Errorf("decode match %d: %w", id, err)
	}

	return &match, nil
}

// ── Write Operations ────────────────────────────────────────────────────

// CreateMatchRequest mirrors the API's request body.
type CreateMatchRequest struct {
	TeamAName string `json:"team_a_name"`
	TeamATag  string `json:"team_a_tag"`
	TeamBName string `json:"team_b_name"`
	TeamBTag  string `json:"team_b_tag"`
	BestOf    int    `json:"best_of"`
	Event     string `json:"event"`
}

// CreateMatch creates a match via the Admin API.
func (c *APIClient) CreateMatch(req CreateMatchRequest) (*Match, error) {
	body, _ := json.Marshal(req)
	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/api/v1/admin/matches",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("POST create match: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("POST create match: status %d: %s", resp.StatusCode, string(respBody))
	}

	var match Match
	if err := json.NewDecoder(resp.Body).Decode(&match); err != nil {
		return nil, fmt.Errorf("decode created match: %w", err)
	}

	return &match, nil
}

// UpdateMatchVault calls PATCH /api/v1/admin/matches/:id/vault to save on-chain IDs.
func (c *APIClient) UpdateMatchVault(id uint, onChainMatchID uint64, vaultAddr string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"on_chain_match_id": onChainMatchID,
		"vault_address":     vaultAddr,
	})
	req, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/api/v1/admin/matches/%d/vault", c.BaseURL, id),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("PATCH match %d vault: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH match %d vault: status %d: %s", id, resp.StatusCode, string(respBody))
	}

	return nil
}

// UpdateMatchStatus calls PATCH /api/v1/admin/matches/:id
func (c *APIClient) UpdateMatchStatus(id uint, status string) error {
	body, _ := json.Marshal(map[string]string{"status": status})
	req, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/api/v1/admin/matches/%d", c.BaseURL, id),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("PATCH match %d: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH match %d: status %d: %s", id, resp.StatusCode, string(respBody))
	}

	return nil
}

// SetResultRequest mirrors the API's request body.
type SetResultRequest struct {
	Source      string `json:"source"`
	MatchStatus string `json:"match_status"`
	Winner      string `json:"winner,omitempty"`
	ScoreA      int    `json:"score_a"`
	ScoreB      int    `json:"score_b"`
	MapCount    int    `json:"map_count"`
}

// SetResult calls POST /api/v1/admin/matches/:id/result
func (c *APIClient) SetResult(matchID uint, req SetResultRequest) error {
	body, _ := json.Marshal(req)
	resp, err := c.HTTPClient.Post(
		fmt.Sprintf("%s/api/v1/admin/matches/%d/result", c.BaseURL, matchID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("POST result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST result: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Ping checks if the API is reachable.
func (c *APIClient) Ping() bool {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/v1/admin/matches?status=open")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
