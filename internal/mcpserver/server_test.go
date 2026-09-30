package mcpserver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/user/mcp-pr-readiness/internal/config"
)

// createTestConfig creates a config pointing to a temporary git repo.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()

	// Initialize a git repo.
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	// Create a minimal Go project.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Project\n"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	reportDir := filepath.Join(dir, "reports")

	return &config.Config{
		RepoPath:       dir,
		BaseBranch:     "main",
		TestCmd:        []string{"go", "test", "./..."},
		TestFormat:     "plain",
		LintCmd:        []string{"go", "vet", "./..."},
		DependencyFile: "go.mod",
		ReportDir:      reportDir,
		Timeouts:       config.TimeoutsConfig{Tests: 30, Lint: 30, Audit: 15, Git: 30},
		MaxOutputBytes: 8192,
		OSVBaseURL:     "https://api.osv.dev",
	}
}

func TestServerToolListing(t *testing.T) {
	cfg := createTestConfig(t)
	srv := New(cfg)

	ctx := context.Background()

	// Use in-memory transport to test without stdio.
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Start server in background.
	go func() {
		if err := srv.MCPServer().Run(ctx, serverTransport); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Create client and connect.
	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "v1.0.0"},
		nil,
	)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// List tools.
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	expectedTools := map[string]bool{
		"get_repo_status":    false,
		"checkout_pr":        false,
		"run_tests":          false,
		"run_linter":         false,
		"audit_dependencies": false,
		"find_todos":         false,
		"save_report":        false,
	}

	for _, tool := range toolsResult.Tools {
		if _, ok := expectedTools[tool.Name]; ok {
			expectedTools[tool.Name] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestServerCallGetRepoStatus(t *testing.T) {
	cfg := createTestConfig(t)
	srv := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		srv.MCPServer().Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "v1.0.0"},
		nil,
	)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// Call get_repo_status.
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_repo_status",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestServerResources(t *testing.T) {
	cfg := createTestConfig(t)
	srv := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		srv.MCPServer().Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "v1.0.0"},
		nil,
	)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// List resources.
	resourcesResult, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}

	if len(resourcesResult.Resources) < 2 {
		t.Errorf("expected at least 2 resources, got %d", len(resourcesResult.Resources))
	}

	// Read conventions resource.
	readResult, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "repo://conventions",
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	if len(readResult.Contents) == 0 {
		t.Fatal("expected content from conventions resource")
	}
}

func TestServerPrompt(t *testing.T) {
	cfg := createTestConfig(t)
	srv := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	go func() {
		srv.MCPServer().Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "v1.0.0"},
		nil,
	)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	// List prompts.
	promptsResult, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}

	if len(promptsResult.Prompts) == 0 {
		t.Fatal("expected at least 1 prompt")
	}

	// Get review_pr prompt.
	getResult, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "review_pr",
		Arguments: map[string]string{"pr_number": "42"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}

	if len(getResult.Messages) == 0 {
		t.Fatal("expected messages in prompt result")
	}
}
