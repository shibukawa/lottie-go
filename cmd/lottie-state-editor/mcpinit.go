package main

// `init` writes the client-side half of an MCP session — the project
// config each coding tool reads — all pointing at one fixed port, so
// "start the editor, start the agent" needs no copying of addresses out
// of the title row.

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	initDefaultPort = 7391
	initDefaultName = "lottie-editor"
)

// mcpClient is one tool that can host the agent: where its project-level
// MCP config lives and how a server entry is spelled there.
type mcpClient struct {
	name    string
	label   string
	aliases []string
	file    string // relative to the project directory
	write   func(o *initOptions, path string) error
	note    string // printed after writing; what the tool needs beyond the file
}

// The entry shapes differ in small ways that each tool insists on:
// Claude Code and VS Code want a type, Cursor and Kiro infer it from url
// vs command, Antigravity spells the URL serverUrl, Codex and Grok Build
// read TOML.
var mcpClients = []mcpClient{
	{
		name: "claude", label: "Claude Code", file: ".mcp.json",
		write: func(o *initOptions, p string) error {
			return mergeJSONServers(p, "mcpServers", o.name, o.entry(entryTyped))
		},
	},
	{
		name: "copilot", label: "GitHub Copilot (VS Code)", aliases: []string{"vscode"}, file: filepath.Join(".vscode", "mcp.json"),
		write: func(o *initOptions, p string) error {
			return mergeJSONServers(p, "servers", o.name, o.entry(entryTyped))
		},
	},
	{
		name: "codex", label: "OpenAI Codex", file: filepath.Join(".codex", "config.toml"),
		write: func(o *initOptions, p string) error { return mergeTOMLServer(p, o.name, o.tomlBody()) },
		note:  "Codex reads .codex/config.toml only for trusted projects; answer its trust prompt once, or add [projects.\"<abs path>\"] trust_level = \"trusted\" to ~/.codex/config.toml",
	},
	{
		name: "kiro", label: "Kiro", file: filepath.Join(".kiro", "settings", "mcp.json"),
		write: func(o *initOptions, p string) error {
			return mergeJSONServers(p, "mcpServers", o.name, o.entry(entryPlain))
		},
	},
	{
		name: "antigravity", label: "Google Antigravity", file: filepath.Join(".agents", "mcp_config.json"),
		write: func(o *initOptions, p string) error {
			return mergeJSONServers(p, "mcpServers", o.name, o.entry(entryServerURL))
		},
	},
	{
		name: "grok", label: "Grok Build", aliases: []string{"grok-build", "grokbuild"}, file: filepath.Join(".grok", "config.toml"),
		write: func(o *initOptions, p string) error { return mergeTOMLServer(p, o.name, o.tomlBody()) },
		note:  "Grok Build also reads .mcp.json and .cursor/mcp.json, so the Claude or Cursor entry alone would have reached it",
	},
	{
		name: "cursor", label: "Cursor", file: filepath.Join(".cursor", "mcp.json"),
		write: func(o *initOptions, p string) error {
			return mergeJSONServers(p, "mcpServers", o.name, o.entry(entryPlain))
		},
	},
}

func clientByName(name string) (*mcpClient, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range mcpClients {
		c := &mcpClients[i]
		if c.name == name {
			return c, true
		}
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return nil, false
}

func clientNames() []string {
	var out []string
	for _, c := range mcpClients {
		out = append(out, c.name)
	}
	return out
}

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
func runInit(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stdout)
	var o initOptions
	fs.StringVar(&o.dir, "dir", ".", "project directory to write the client configs into")
	fs.IntVar(&o.port, "port", initDefaultPort, "loopback port the editor will listen on")
	fs.StringVar(&o.name, "name", initDefaultName, "server name as the agent sees it")
	fs.StringVar(&o.transport, "transport", "http", "http: the agent connects to a running editor; stdio: the agent launches the editor on the bundle")
	fs.StringVar(&o.clients, "clients", "", "comma-separated tools to set up: "+strings.Join(clientNames(), ", ")+", or all; asked interactively when omitted")
	fs.Usage = func() {
		fmt.Fprintln(stdout, "usage: lottie-state-editor init [flags] [bundle.lottie]")
		fmt.Fprintln(stdout, "writes each tool's project MCP config pointing at this editor, then prints how to start it")
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
	clients, err := o.pickClients(stdin, stdout)
	if err != nil {
		return err
	}
	return o.run(clients, stdout)
}

// pickClients resolves -clients, asking on the terminal when it was left
// out — the tools differ per developer, and a default set would write
// files for tools that are not installed.
func (o *initOptions) pickClients(stdin io.Reader, stdout io.Writer) ([]*mcpClient, error) {
	spec := strings.TrimSpace(o.clients)
	if spec == "" {
		if !isTerminal(stdin) {
			return nil, errors.New("-clients is required (" + strings.Join(clientNames(), ", ") + ", or all)")
		}
		fmt.Fprintln(stdout, "Which tools should connect to the editor?")
		for i, c := range mcpClients {
			fmt.Fprintf(stdout, "  %d) %-12s %s  →  %s\n", i+1, c.name, c.label, c.file)
		}
		fmt.Fprint(stdout, "numbers or names, comma-separated, or all: ")
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		spec = strings.TrimSpace(line)
		if spec == "" {
			return nil, errors.New("no tools chosen")
		}
	}
	var out []*mcpClient
	seen := map[string]bool{}
	add := func(c *mcpClient) {
		if !seen[c.name] {
			seen[c.name] = true
			out = append(out, c)
		}
	}
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "all") {
			for i := range mcpClients {
				add(&mcpClients[i])
			}
			continue
		}
		if n, err := strconv.Atoi(item); err == nil {
			if n < 1 || n > len(mcpClients) {
				return nil, fmt.Errorf("no tool number %d", n)
			}
			add(&mcpClients[n-1])
			continue
		}
		c, ok := clientByName(item)
		if !ok {
			return nil, fmt.Errorf("unknown tool %q (%s, or all)", item, strings.Join(clientNames(), ", "))
		}
		add(c)
	}
	if len(out) == 0 {
		return nil, errors.New("no tools chosen")
	}
	return out, nil
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func (o *initOptions) url() string {
	return "http://127.0.0.1:" + strconv.Itoa(o.port) + "/mcp"
}

type entryStyle int

const (
	entryTyped     entryStyle = iota // {"type":"http","url":...} / {"type":"stdio",...}
	entryPlain                       // {"url":...} / {"command":...}; the tool infers the transport
	entryServerURL                   // Antigravity: {"serverUrl":...}
)

// entry is the JSON server record in the style a tool expects.
func (o *initOptions) entry(style entryStyle) map[string]any {
	if o.transport == "stdio" {
		e := map[string]any{"command": o.exe, "args": o.stdioArgs()}
		if style == entryTyped {
			e["type"] = "stdio"
		}
		return e
	}
	switch style {
	case entryTyped:
		return map[string]any{"type": "http", "url": o.url()}
	case entryServerURL:
		return map[string]any{"serverUrl": o.url()}
	}
	return map[string]any{"url": o.url()}
}

// tomlBody is the [mcp_servers.<name>] table Codex and Grok Build read.
func (o *initOptions) tomlBody() string {
	if o.transport == "stdio" {
		var args []string
		for _, a := range o.stdioArgs() {
			args = append(args, strconv.Quote(a))
		}
		return "command = " + strconv.Quote(o.exe) + "\nargs = [" + strings.Join(args, ", ") + "]\n"
	}
	return "url = " + strconv.Quote(o.url()) + "\n"
}

func (o *initOptions) stdioArgs() []string {
	return []string{"-mcp", "stdio", o.bundlePath()}
}

// bundlePath is absolute, because a stdio launcher's working directory
// is the agent's, not the editor's.
func (o *initOptions) bundlePath() string {
	if abs, err := filepath.Abs(o.bundle); err == nil {
		return abs
	}
	return o.bundle
}

func (o *initOptions) run(clients []*mcpClient, stdout io.Writer) error {
	for _, c := range clients {
		p := filepath.Join(o.dir, c.file)
		if err := c.write(o, p); err != nil {
			return fmt.Errorf("%s: %w", c.label, err)
		}
		fmt.Fprintf(stdout, "%-12s wrote %s\n", c.name, p)
		if c.note != "" {
			fmt.Fprintf(stdout, "%-12s note: %s\n", "", c.note)
		}
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

// mergeTOMLServer writes the [mcp_servers.<name>] table into the TOML file
// at path: replacing that table when the file already has one, appending
// otherwise, and leaving every other line alone. A textual merge, because
// the file is the tool's and may hold more than this program understands.
func mergeTOMLServer(path, name, body string) error {
	header := "[mcp_servers." + tomlKey(name) + "]"
	block := header + "\n" + body
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(existing)
	if i := strings.Index(text, header); i >= 0 && (i == 0 || text[i-1] == '\n') {
		// Cut from our header to the next table header.
		rest := text[i+len(header):]
		end := len(text)
		if loc := tomlHeader.FindStringIndex(rest); loc != nil {
			end = i + len(header) + loc[0]
		}
		text = text[:i] + block + text[end:]
	} else {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if text != "" {
			text += "\n"
		}
		text += block
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

var tomlHeader = regexp.MustCompile(`(?m)^\s*\[`)

// tomlKey quotes a table key that is not a bare key.
func tomlKey(name string) string {
	for _, r := range name {
		if !(r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return strconv.Quote(name)
		}
	}
	return name
}
