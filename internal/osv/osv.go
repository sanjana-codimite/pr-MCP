// Package osv provides a client for the OSV.dev vulnerability API.
//
// This is the external API integration for the PR Readiness server.
// It queries OSV.dev for known vulnerabilities in Go module dependencies.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client talks to the OSV.dev API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new OSV.dev API client with the given base URL and timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// QueryBatchRequest is the request body for POST /v1/querybatch.
type QueryBatchRequest struct {
	Queries []QueryBatchEntry `json:"queries"`
}

// QueryBatchEntry is a single query in the batch.
type QueryBatchEntry struct {
	Package QueryPackage `json:"package"`
	Version string       `json:"version"`
}

// QueryPackage identifies a package in the OSV ecosystem.
type QueryPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// QueryBatchResponse is the response from POST /v1/querybatch.
type QueryBatchResponse struct {
	Results []QueryBatchResult `json:"results"`
}

// QueryBatchResult is a single result from the batch query.
type QueryBatchResult struct {
	Vulns []VulnRef `json:"vulns"`
}

// VulnRef is a reference to a vulnerability returned by querybatch.
type VulnRef struct {
	ID string `json:"id"`
}

// Vulnerability contains the details of a single vulnerability.
type Vulnerability struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Aliases  []string `json:"aliases"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	DatabaseSpecific *DatabaseSpecific `json:"database_specific"`
}

// DatabaseSpecific contains database-specific fields.
type DatabaseSpecific struct {
	Severity string `json:"severity"`
}

// ModuleVulnerability pairs a module with its found vulnerabilities.
type ModuleVulnerability struct {
	Module   string
	Version  string
	VulnID   string
	Severity string
	Summary  string
}

// FormatGoVersion ensures the version has the "v" prefix that OSV expects
// for the Go ecosystem.
func FormatGoVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// QueryBatch sends a batch query to OSV.dev for the given modules.
func (c *Client) QueryBatch(ctx context.Context, modules []QueryBatchEntry) (*QueryBatchResponse, error) {
	body := QueryBatchRequest{Queries: modules}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling query: %w", err)
	}

	url := c.BaseURL + "/v1/querybatch"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mcp-pr-readiness/1.0.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSV querybatch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("OSV querybatch returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result QueryBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding querybatch response: %w", err)
	}

	return &result, nil
}

// GetVulnerability fetches details for a single vulnerability by ID.
func (c *Client) GetVulnerability(ctx context.Context, id string) (*Vulnerability, error) {
	url := fmt.Sprintf("%s/v1/vulns/%s", c.BaseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "mcp-pr-readiness/1.0.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSV vulns request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("OSV vulns returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var vuln Vulnerability
	if err := json.NewDecoder(resp.Body).Decode(&vuln); err != nil {
		return nil, fmt.Errorf("decoding vuln response: %w", err)
	}

	return &vuln, nil
}

// FetchVulnDetails fetches details for multiple vulnerability IDs concurrently.
// Max 50 IDs, concurrency <= 5.
func (c *Client) FetchVulnDetails(ctx context.Context, ids []string) ([]ModuleVulnerability, error) {
	// Deduplicate IDs.
	seen := make(map[string]bool)
	var uniqueIDs []string
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	// Cap at 50.
	if len(uniqueIDs) > 50 {
		uniqueIDs = uniqueIDs[:50]
	}

	var (
		mu      sync.Mutex
		vulns   []ModuleVulnerability
		sem     = make(chan struct{}, 5) // concurrency limit
		wg      sync.WaitGroup
		firstErr error
	)

	for _, id := range uniqueIDs {
		wg.Add(1)
		go func(vulnID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			v, err := c.GetVulnerability(ctx, vulnID)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}

			severity := "UNKNOWN"
			if v.DatabaseSpecific != nil && v.DatabaseSpecific.Severity != "" {
				severity = strings.ToUpper(v.DatabaseSpecific.Severity)
			}

			vulns = append(vulns, ModuleVulnerability{
				VulnID:   v.ID,
				Severity: severity,
				Summary:  v.Summary,
			})
		}(id)
	}

	wg.Wait()
	return vulns, firstErr
}

// ClassifySeverity normalizes severity strings.
func ClassifySeverity(sev string) string {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return "CRITICAL"
	case "HIGH":
		return "HIGH"
	case "MODERATE", "MEDIUM":
		return "MODERATE"
	case "LOW":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
