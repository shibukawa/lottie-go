package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesClientConfigs(t *testing.T) {
	dir := t.TempDir()
	// An existing .mcp.json with another server survives the merge.
	os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{"other":{"type":"http","url":"http://x/mcp"}}}`), 0o644)

	var out bytes.Buffer
	if err := runInit([]string{"-dir", dir, "-port", "7400", "character.lottie"}, &out); err != nil {
		t.Fatal(err)
	}
	var claude map[string]map[string]map[string]any
	data, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err := json.Unmarshal(data, &claude); err != nil {
		t.Fatal(err)
	}
	if claude["mcpServers"]["other"] == nil {
		t.Fatalf("merge dropped the existing server: %s", data)
	}
	if got := claude["mcpServers"]["lottie-editor"]["url"]; got != "http://127.0.0.1:7400/mcp" {
		t.Fatalf("claude url = %v", got)
	}
	data, _ = os.ReadFile(filepath.Join(dir, ".vscode", "mcp.json"))
	var vscode map[string]map[string]map[string]any
	if err := json.Unmarshal(data, &vscode); err != nil {
		t.Fatal(err)
	}
	if got := vscode["servers"]["lottie-editor"]["type"]; got != "http" {
		t.Fatalf("vscode entry = %v", vscode["servers"])
	}
	text := out.String()
	for _, want := range []string{"codex mcp add lottie-editor --url http://127.0.0.1:7400/mcp", "-mcp 127.0.0.1:7400 character.lottie"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

func TestInitStdioNeedsABundleAndRecordsTheLauncher(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runInit([]string{"-dir", dir, "-transport", "stdio"}, &out); err == nil {
		t.Fatalf("stdio without a bundle accepted")
	}
	if err := runInit([]string{"-dir", dir, "-transport", "stdio", "-clients", "claude", "hero.lottie"}, &out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	var claude map[string]map[string]map[string]any
	json.Unmarshal(data, &claude)
	entry := claude["mcpServers"]["lottie-editor"]
	args, _ := entry["args"].([]any)
	if entry["type"] != "stdio" || len(args) != 3 || args[0] != "-mcp" || args[1] != "stdio" || !filepath.IsAbs(args[2].(string)) {
		t.Fatalf("stdio entry = %v", entry)
	}
	if _, err := os.Stat(filepath.Join(dir, ".vscode", "mcp.json")); err == nil {
		t.Fatalf("vscode config written although only claude was asked for")
	}
}
