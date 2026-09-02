package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var doc map[string]map[string]map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, data)
	}
	return doc
}

func TestInitWritesEveryClientInItsOwnShape(t *testing.T) {
	dir := t.TempDir()
	// An existing .mcp.json with another server survives the merge.
	os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{"other":{"type":"http","url":"http://x/mcp"}}}`), 0o644)

	var out bytes.Buffer
	if err := runInit([]string{"-dir", dir, "-port", "7400", "-clients", "all", "character.lottie"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:7400/mcp"

	claude := readJSON(t, filepath.Join(dir, ".mcp.json"))["mcpServers"]
	if claude["other"] == nil || claude["lottie-editor"]["url"] != url || claude["lottie-editor"]["type"] != "http" {
		t.Fatalf("claude = %v", claude)
	}
	copilot := readJSON(t, filepath.Join(dir, ".vscode", "mcp.json"))["servers"]
	if copilot["lottie-editor"]["type"] != "http" || copilot["lottie-editor"]["url"] != url {
		t.Fatalf("copilot = %v", copilot)
	}
	for _, p := range []string{filepath.Join(".cursor", "mcp.json"), filepath.Join(".kiro", "settings", "mcp.json")} {
		e := readJSON(t, filepath.Join(dir, p))["mcpServers"]["lottie-editor"]
		if e["url"] != url || e["type"] != nil {
			t.Fatalf("%s = %v", p, e)
		}
	}
	anti := readJSON(t, filepath.Join(dir, ".agents", "mcp_config.json"))["mcpServers"]["lottie-editor"]
	if anti["serverUrl"] != url || anti["url"] != nil {
		t.Fatalf("antigravity = %v", anti)
	}
	for _, p := range []string{filepath.Join(".codex", "config.toml"), filepath.Join(".grok", "config.toml")} {
		data, _ := os.ReadFile(filepath.Join(dir, p))
		if want := "[mcp_servers.lottie-editor]\nurl = \"" + url + "\"\n"; string(data) != want {
			t.Fatalf("%s = %q", p, data)
		}
	}
	text := out.String()
	for _, want := range []string{"codex", "trusted projects", "-mcp 127.0.0.1:7400 character.lottie"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q:\n%s", want, text)
		}
	}
}

func TestInitTOMLMergeReplacesOnlyOurTable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".codex", "config.toml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("model = \"gpt-5\"\n\n[mcp_servers.lottie-editor]\nurl = \"http://old/mcp\"\n\n[mcp_servers.other]\ncommand = \"x\"\n"), 0o644)
	if err := runInit([]string{"-dir", dir, "-clients", "codex"}, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	text := string(data)
	if strings.Contains(text, "http://old/mcp") || !strings.Contains(text, "url = \"http://127.0.0.1:7391/mcp\"") {
		t.Fatalf("our table not replaced:\n%s", text)
	}
	if !strings.HasPrefix(text, "model = \"gpt-5\"") || !strings.Contains(text, "[mcp_servers.other]\ncommand = \"x\"") {
		t.Fatalf("other content damaged:\n%s", text)
	}
}

func TestInitSelectsClientsByNumberNameAndAlias(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runInit([]string{"-dir", dir, "-clients", "1, vscode, grok-build"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".mcp.json", filepath.Join(".vscode", "mcp.json"), filepath.Join(".grok", "config.toml")} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s not written", p)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "mcp.json")); err == nil {
		t.Errorf("cursor written although not chosen")
	}
	if err := runInit([]string{"-dir", dir, "-clients", "emacs"}, strings.NewReader(""), &out); err == nil || !strings.Contains(err.Error(), "emacs") {
		t.Fatalf("unknown tool accepted: %v", err)
	}
	// Without a terminal, leaving -clients out is an error rather than a
	// silent default set.
	if err := runInit([]string{"-dir", dir}, strings.NewReader(""), &out); err == nil {
		t.Fatalf("missing -clients accepted without a terminal")
	}
}

func TestInitStdioNeedsABundleAndRecordsTheLauncher(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runInit([]string{"-dir", dir, "-transport", "stdio", "-clients", "claude"}, strings.NewReader(""), &out); err == nil {
		t.Fatalf("stdio without a bundle accepted")
	}
	if err := runInit([]string{"-dir", dir, "-transport", "stdio", "-clients", "claude,codex,antigravity", "hero.lottie"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	entry := readJSON(t, filepath.Join(dir, ".mcp.json"))["mcpServers"]["lottie-editor"]
	args, _ := entry["args"].([]any)
	if entry["type"] != "stdio" || len(args) != 3 || args[0] != "-mcp" || args[1] != "stdio" || !filepath.IsAbs(args[2].(string)) {
		t.Fatalf("stdio entry = %v", entry)
	}
	anti := readJSON(t, filepath.Join(dir, ".agents", "mcp_config.json"))["mcpServers"]["lottie-editor"]
	if anti["command"] == nil || anti["type"] != nil {
		t.Fatalf("antigravity stdio entry = %v", anti)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if !strings.Contains(string(data), "command = ") || !strings.Contains(string(data), "args = [\"-mcp\", \"stdio\", ") {
		t.Fatalf("codex stdio table = %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".vscode", "mcp.json")); err == nil {
		t.Fatalf("vscode config written although not asked for")
	}
}
