package main

// `init` writes the client-side half of an MCP session: the project
// config files Claude Code and VS Code read, and the command Codex wants,
// all pointing at one fixed port — so "start the editor, start the agent"
// needs no copying of addresses out of the title row.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	initDefaultPort = 7391
	initDefaultName = "lottie-editor"
)

type initOptions struct {
	dir       string
	port      int
	name      string
	transport string
	clients   string
	bundle    string
	exe       string
}

// runInit handles `lottie-state-editor init [flags] [bundle.lottie]`.
func runInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stdout)
	var o initOptions
	fs.StringVar(&o.dir, "dir", ".", "project directory to write the client configs into")
	fs.IntVar(&o.port, "port", initDefaultPort, "loopback port the editor will listen on")
	fs.StringVar(&o.name, "name", initDefaultName, "server name as the agent sees it")
	fs.StringVar(&o.transport, "transport", "http", "http: the agent connects to a running editor; stdio: the agent launches the editor on the bundle")
	fs.StringVar(&o.clients, "clients", "claude,vscode,codex", "comma-separated: claude (.mcp.json), vscode (.vscode/mcp.json), codex (prints the command)")
	fs.Usage = func() {
		fmt.Fprintln(stdout, "usage: lottie-state-editor init [flags] [bundle.lottie]")
		fmt.Fprintln(stdout, "writes MCP client configs pointing at this editor, then prints how to start it")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("init takes at most one bundle path")
	}
	if fs.NArg() == 1 {
		o.bundle = fs.Arg(0)
	}
	if o.transport != "http" && o.transport != "stdio" {
		return fmt.Errorf("transport must be http or stdio, not %q", o.transport)
	}
	if o.transport == "stdio" && o.bundle == "" {
		return errors.New("stdio needs the bundle path: the agent launches the editor on it")
	}
	if o.port <= 0 || o.port > 65535 {
		return fmt.Errorf("port %d out of range", o.port)
	}
	// Under `go run` the executable is a throwaway under the build cache;
	// naming it would record a path that is gone by the next run.
	exe, err := os.Executable()
	if err != nil || strings.Contains(exe, "go-build") {
		exe = "lottie-state-editor"
	}
	o.exe = exe
	return o.run(stdout)
}

func (o *initOptions) url() string {
	return "http://127.0.0.1:" + strconv.Itoa(o.port) + "/mcp"
}

// entry is the server record every client understands, modulo its
// wrapper key.
func (o *initOptions) entry() map[string]any {
	if o.transport == "stdio" {
		return map[string]any{
			"type":    "stdio",
			"command": o.exe,
			"args":    []string{"-mcp", "stdio", o.bundlePath()},
		}
	}
	return map[string]any{"type": "http", "url": o.url()}
}

// bundlePath is absolute, because a stdio launcher's working directory
// is the agent's, not the editor's.
func (o *initOptions) bundlePath() string {
	if abs, err := filepath.Abs(o.bundle); err == nil {
		return abs
	}
	return o.bundle
}

func (o *initOptions) run(stdout io.Writer) error {
	var written []string
	for _, c := range strings.Split(o.clients, ",") {
		c = strings.TrimSpace(strings.ToLower(c))
		switch c {
		case "":
		case "claude":
			p := filepath.Join(o.dir, ".mcp.json")
			if err := mergeJSONServers(p, "mcpServers", o.name, o.entry()); err != nil {
				return err
			}
			written = append(written, p)
		case "vscode":
			p := filepath.Join(o.dir, ".vscode", "mcp.json")
			if err := mergeJSONServers(p, "servers", o.name, o.entry()); err != nil {
				return err
			}
			written = append(written, p)
		case "codex":
			// Codex keeps its servers in ~/.codex/config.toml, which is
			// not the project's to write; its own command does the merge.
			if o.transport == "stdio" {
				fmt.Fprintf(stdout, "codex: codex mcp add %s -- %s -mcp stdio %s\n", o.name, o.exe, o.bundlePath())
			} else {
				fmt.Fprintf(stdout, "codex: codex mcp add %s --url %s\n", o.name, o.url())
			}
		default:
			return fmt.Errorf("unknown client %q (claude, vscode, codex)", c)
		}
	}
	for _, p := range written {
		fmt.Fprintf(stdout, "wrote %s\n", p)
	}
	if o.transport == "stdio" {
		fmt.Fprintf(stdout, "the agent starts the editor itself; the window opens on %s\n", o.bundlePath())
		return nil
	}
	launch := o.exe + " -mcp 127.0.0.1:" + strconv.Itoa(o.port)
	if o.bundle != "" {
		launch += " " + o.bundle
	}
	fmt.Fprintf(stdout, "start the editor with:\n  %s\nthen start the agent; it connects to %s\n", launch, o.url())
	return nil
}

// mergeJSONServers sets servers[name] in the JSON object at path, keeping
// every other member the file already has.
func mergeJSONServers(path, wrapper, name string, entry map[string]any) error {
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("%s exists but is not a JSON object: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	servers, _ := doc[wrapper].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[name] = entry
	doc[wrapper] = servers
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
