package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	serverPath := flag.String("server", "./bin/prready-server", "Path to the MCP server binary")
	configPath := flag.String("config", "./.prready.json", "Path to the server config file")
	flag.Parse()

	ctx := context.Background()

	fmt.Println("=== PR Readiness MCP Client Demo ===")
	fmt.Println()

	// Create a client and connect to the server via command transport.
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "prready-client",
			Version: "v1.0.0",
		},
		nil,
	)

	subcmd := exec.Command(*serverPath, "--config", *configPath)
	subcmd.Stderr = os.Stderr
	transport := &mcp.CommandTransport{
		Command: subcmd,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer session.Close()

	fmt.Println("Connected to server successfully!")
	fmt.Println()

	// List tools.
	fmt.Println("--- Tools ---")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("ListTools: %v", err)
	}
	for _, tool := range toolsResult.Tools {
		fmt.Printf("  • %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// List resources.
	fmt.Println("--- Resources ---")
	resourcesResult, err := session.ListResources(ctx, nil)
	if err != nil {
		log.Fatalf("ListResources: %v", err)
	}
	for _, res := range resourcesResult.Resources {
		fmt.Printf("  • %s (%s): %s\n", res.URI, res.MIMEType, res.Description)
	}
	fmt.Println()

	// List prompts.
	fmt.Println("--- Prompts ---")
	promptsResult, err := session.ListPrompts(ctx, nil)
	if err != nil {
		log.Fatalf("ListPrompts: %v", err)
	}
	for _, p := range promptsResult.Prompts {
		fmt.Printf("  • %s: %s\n", p.Name, p.Description)
	}
	fmt.Println()

	// Call get_repo_status.
	fmt.Println("--- Calling get_repo_status ---")
	statusResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_repo_status",
	})
	if err != nil {
		log.Fatalf("CallTool(get_repo_status): %v", err)
	}
	printToolResult(statusResult)

	// Call run_tests.
	fmt.Println("--- Calling run_tests ---")
	testsResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "run_tests",
	})
	if err != nil {
		log.Fatalf("CallTool(run_tests): %v", err)
	}
	printToolResult(testsResult)

	// Read repo://conventions resource.
	fmt.Println("--- Reading repo://conventions ---")
	convResult, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "repo://conventions",
	})
	if err != nil {
		log.Fatalf("ReadResource(repo://conventions): %v", err)
	}
	for _, c := range convResult.Contents {
		text := c.Text
		if len(text) > 500 {
			text = text[:500] + "... (truncated)"
		}
		fmt.Println(text)
	}
	fmt.Println()

	// Get review_pr prompt.
	fmt.Println("--- Getting review_pr prompt for PR #1 ---")
	promptResult, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "review_pr",
		Arguments: map[string]string{"pr_number": "1"},
	})
	if err != nil {
		log.Fatalf("GetPrompt(review_pr): %v", err)
	}
	fmt.Printf("Description: %s\n", promptResult.Description)
	for _, msg := range promptResult.Messages {
		text := ""
		if tc, ok := msg.Content.(*mcp.TextContent); ok {
			text = tc.Text
		}
		if len(text) > 500 {
			text = text[:500] + "... (truncated)"
		}
		fmt.Printf("Role: %s\nContent:\n%s\n", msg.Role, text)
	}
	fmt.Println()

	fmt.Println("=== Demo complete ===")
}

func printToolResult(result *mcp.CallToolResult) {
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			// Pretty-print JSON.
			var obj interface{}
			if err := json.Unmarshal([]byte(tc.Text), &obj); err == nil {
				pretty, _ := json.MarshalIndent(obj, "  ", "  ")
				fmt.Println("  " + string(pretty))
			} else {
				fmt.Println("  " + tc.Text)
			}
		}
	}
	fmt.Println()
}
