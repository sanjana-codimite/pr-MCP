package osv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryBatch_NoVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/querybatch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		// Return empty results.
		json.NewEncoder(w).Encode(QueryBatchResponse{
			Results: []QueryBatchResult{{Vulns: nil}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	resp, err := client.QueryBatch(context.Background(), []QueryBatchEntry{
		{Package: QueryPackage{Name: "golang.org/x/text", Ecosystem: "Go"}, Version: "v0.14.0"},
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(resp.Results))
	}
	if len(resp.Results[0].Vulns) != 0 {
		t.Errorf("expected no vulns, got %d", len(resp.Results[0].Vulns))
	}
}

func TestQueryBatch_WithVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(QueryBatchResponse{
			Results: []QueryBatchResult{
				{Vulns: []VulnRef{{ID: "GO-2023-0001"}, {ID: "GO-2023-0002"}}},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	resp, err := client.QueryBatch(context.Background(), []QueryBatchEntry{
		{Package: QueryPackage{Name: "golang.org/x/text", Ecosystem: "Go"}, Version: "v0.3.0"},
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	if len(resp.Results[0].Vulns) != 2 {
		t.Errorf("got %d vulns, want 2", len(resp.Results[0].Vulns))
	}
}

func TestQueryBatch_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	_, err := client.QueryBatch(context.Background(), []QueryBatchEntry{
		{Package: QueryPackage{Name: "example.com/mod", Ecosystem: "Go"}, Version: "v1.0.0"},
	})
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestQueryBatch_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than client timeout.
		json.NewEncoder(w).Encode(QueryBatchResponse{})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 500*time.Millisecond)
	_, err := client.QueryBatch(context.Background(), []QueryBatchEntry{
		{Package: QueryPackage{Name: "example.com/mod", Ecosystem: "Go"}, Version: "v1.0.0"},
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestQueryBatch_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	_, err := client.QueryBatch(context.Background(), []QueryBatchEntry{
		{Package: QueryPackage{Name: "example.com/mod", Ecosystem: "Go"}, Version: "v1.0.0"},
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestGetVulnerability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/vulns/GO-2023-0001" {
			json.NewEncoder(w).Encode(Vulnerability{
				ID:      "GO-2023-0001",
				Summary: "Test vulnerability",
				Aliases: []string{"CVE-2023-0001"},
				DatabaseSpecific: &DatabaseSpecific{
					Severity: "HIGH",
				},
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	vuln, err := client.GetVulnerability(context.Background(), "GO-2023-0001")
	if err != nil {
		t.Fatalf("GetVulnerability: %v", err)
	}
	if vuln.ID != "GO-2023-0001" {
		t.Errorf("ID = %q, want %q", vuln.ID, "GO-2023-0001")
	}
	if vuln.Summary != "Test vulnerability" {
		t.Errorf("Summary = %q, want %q", vuln.Summary, "Test vulnerability")
	}
	if vuln.DatabaseSpecific.Severity != "HIGH" {
		t.Errorf("Severity = %q, want %q", vuln.DatabaseSpecific.Severity, "HIGH")
	}
}

func TestFormatGoVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.0.0", "v1.0.0"},
		{"1.0.0", "v1.0.0"},
		{"v0.3.0", "v0.3.0"},
		{"0.3.0", "v0.3.0"},
	}
	for _, tt := range tests {
		got := FormatGoVersion(tt.input)
		if got != tt.want {
			t.Errorf("FormatGoVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"CRITICAL", "CRITICAL"},
		{"HIGH", "HIGH"},
		{"MODERATE", "MODERATE"},
		{"MEDIUM", "MODERATE"},
		{"LOW", "LOW"},
		{"unknown", "UNKNOWN"},
		{"", "UNKNOWN"},
	}
	for _, tt := range tests {
		got := ClassifySeverity(tt.input)
		if got != tt.want {
			t.Errorf("ClassifySeverity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFetchVulnDetails_MixedSeverities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/vulns/GO-2023-0001":
			json.NewEncoder(w).Encode(Vulnerability{
				ID: "GO-2023-0001", Summary: "High severity vuln",
				DatabaseSpecific: &DatabaseSpecific{Severity: "HIGH"},
			})
		case "/v1/vulns/GO-2023-0002":
			json.NewEncoder(w).Encode(Vulnerability{
				ID: "GO-2023-0002", Summary: "Low severity vuln",
				DatabaseSpecific: &DatabaseSpecific{Severity: "LOW"},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 10*time.Second)
	vulns, err := client.FetchVulnDetails(context.Background(), []string{"GO-2023-0001", "GO-2023-0002"})
	if err != nil {
		t.Fatalf("FetchVulnDetails: %v", err)
	}
	if len(vulns) != 2 {
		t.Fatalf("got %d vulns, want 2", len(vulns))
	}

	// Check that we have both severities.
	hasHigh, hasLow := false, false
	for _, v := range vulns {
		if v.Severity == "HIGH" {
			hasHigh = true
		}
		if v.Severity == "LOW" {
			hasLow = true
		}
	}
	if !hasHigh || !hasLow {
		t.Errorf("expected HIGH and LOW severities, got: %+v", vulns)
	}
}
