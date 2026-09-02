package main

// The editor as an MCP server. A coding agent drives the same Model the
// panes drive — the Model knows nothing about widgets, so this is a third
// front end over it, beside the menus and the inspector — while the author
// watches the window and can step in at any time.
//
// Every tool call is a closure queued for the game loop and run from
// Root.Tick, the way file dialog results are applied: the transport's
// goroutines never touch the Model. A call blocks until its closure has
// run, so the reply reflects the document after the edit.
//
// Reply shape, shared by every tool: {focus, generation, changed, problems,
// status, ...payload}. Refusals are {error, hint, valid} with IsError set,
// so the agent sees the valid choices and corrects in one retry.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpVersion = "0.1.0"

// mcpServer owns the protocol side: the SDK server, the transports, and the
// queue that hands calls to the game loop.
type mcpServer struct {
	model  *Model
	server *mcp.Server
	queue  chan func()

	// guard is the selected guard of the selected transition. The
	// inspector keeps that index in its own widget, so the cursor keeps
	// one too; -1 is none.
	guard int
	// selInput marks that the last select landed on an input: inputs are
	// selected in the Model but are not an inspect target, so the focus
	// cannot read them back from the Model alone.
	selInput bool

	// capture is served from the game wrapper's Draw (mcprender.go).
	capture chan *captureReq

	mu      sync.Mutex
	httpSrv *http.Server
	url     string
	stdio   bool
	cancel  context.CancelFunc
}

// refusal is a tool error the agent can act on: what was wrong, what to
// do, and the choices that would have been accepted.
type refusal struct {
	Message string   `json:"error"`
	Hint    string   `json:"hint,omitempty"`
	Valid   []string `json:"valid,omitempty"`
}

func (r *refusal) Error() string { return r.Message }

func refuse(msg string, hint string, valid ...string) error {
	return &refusal{Message: msg, Hint: hint, Valid: valid}
}

func refusef(format string, args ...any) error {
	return &refusal{Message: fmt.Sprintf(format, args...)}
}

func newMCPServer(m *Model) *mcpServer {
	s := &mcpServer{
		model:   m,
		queue:   make(chan func(), 16),
		guard:   -1,
		capture: make(chan *captureReq, 1),
	}
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "lottie-state-editor",
		Title:   "Lottie State Machine Editor",
		Version: mcpVersion,
	}, &mcp.ServerOptions{
		Instructions: mcpInstructions,
	})
	s.registerTools()
	s.registerResources()
	return s
}

const mcpInstructions = `This server is a running Lottie state machine editor window. You and the
human share ONE selection (the focus): what is on stage, the parked
playhead, and the thing the inspector edits. Work in this loop:
describe -> select <address> -> inspect (the form lists the fields you can
set, their options and whether they are keyed) -> set / add / remove / move
/ pose / path -> render (look at the picture) -> validate -> file save.
Addresses: clip:<id>, machine:<id>, state:<machine>/<name>,
transition:<machine>/<state>/<n>, guard:<machine>/<state>/<n>/<g>,
input:<machine>/<name>, part:<clip>/<layer>, key:<clip>/<frame> or
key:<clip>/<layer>/<frame>, layer:<clip>/<layer>, shape:<clip>/<layer>/<i>/<j>,
vertex:<shape>/<v>, stop:<shape>/<n>, uv:<shape>/<v>, hitbox:<clip>/<name>,
span:<clip>/<name>/<n>, body:<n>, socket:<name>, config. A segment may be
omitted to mean the current stage clip or machine; a layer may be named
#<index>. Every reply echoes the focus and the document generation; pass
expect_generation on edits to be refused when the human edited in between.`

// tick drains the queue on the game loop. Called from Root.Tick.
func (s *mcpServer) tick() {
	for {
		select {
		case f := <-s.queue:
			f()
		default:
			return
		}
	}
}

// call runs f on the game loop and waits for it.
func (s *mcpServer) call(ctx context.Context, f func()) error {
	done := make(chan struct{})
	job := func() {
		defer close(done)
		f()
	}
	select {
	case s.queue <- job:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return errors.New("editor game loop is not draining calls")
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---- transports ----

// start brings the server up on spec: "stdio", or a TCP address. A bare
// port or ":port" binds loopback; "host:0" lets the OS pick a port, which
// the status bar and Config pane then show.
func (s *mcpServer) start(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	if spec == "stdio" {
		return s.startStdio()
	}
	return s.startHTTP(spec)
}

func (s *mcpServer) startStdio() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdio {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.stdio, s.cancel = true, cancel
	go func() {
		if err := s.server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, "mcp stdio:", err)
		}
	}()
	s.model.setMCP("stdio", true)
	return nil
}

func (s *mcpServer) startHTTP(spec string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpSrv != nil {
		return nil
	}
	host, port, err := net.SplitHostPort(spec)
	if err != nil {
		// A bare number is a port.
		if _, perr := net.LookupPort("tcp", spec); perr == nil {
			host, port = "", spec
		} else {
			return fmt.Errorf("mcp address %q: %w", spec, err)
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.server },
		&mcp.StreamableHTTPOptions{Stateless: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	srv := &http.Server{Handler: mux}
	s.httpSrv = srv
	s.url = "http://" + ln.Addr().String() + "/mcp"
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "mcp http:", err)
		}
	}()
	s.model.setMCP(s.url, true)
	return nil
}

// stop shuts the HTTP transport down. A stdio session belongs to whoever
// launched the editor and is left alone.
func (s *mcpServer) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv := s.httpSrv; srv != nil {
		s.httpSrv, s.url = nil, ""
		// stop runs on the game loop (the Config pane toggles it), and a
		// call in flight is waiting for that same loop to drain the queue;
		// shutting down synchronously would hold the loop for the whole
		// grace period. Let the reply go out and close afterwards.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				srv.Close()
			}
		}()
	}
	if s.stdio {
		return
	}
	s.model.setMCP("", false)
}

// URL is the HTTP endpoint, or "" while it is down.
func (s *mcpServer) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.url
}

// ---- reply envelope ----

// envelope is what every tool reply starts from.
func (s *mcpServer) envelope(payload map[string]any, changed []string) map[string]any {
	m := s.model
	out := map[string]any{}
	for k, v := range payload {
		out[k] = v
	}
	out["focus"] = s.focus()
	out["generation"] = m.DocGeneration()
	if len(changed) > 0 {
		out["changed"] = changed
	}
	if p := m.Problems(); len(p) > 0 {
		out["problems"] = p
	}
	if st := m.Status(); st != "" {
		out["status"] = st
	}
	return out
}

func jsonText(v any) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

func refusalResult(err error) *mcp.CallToolResult {
	var r *refusal
	if !errors.As(err, &r) {
		r = &refusal{Message: err.Error()}
	}
	b, _ := json.Marshal(r)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: true,
	}
}

// editArgs are the parameters every mutating tool shares. Embedding it
// gives a tool the target and the optimistic-lock check.
type editArgs struct {
	Target           string `json:"target,omitempty" jsonschema:"address to act on; omitted means the current focus"`
	ExpectGeneration *int   `json:"expect_generation,omitempty" jsonschema:"refuse when the document generation is not this value, i.e. the human edited in between"`
}

func (e editArgs) editParams() editArgs { return e }

type edited interface{ editParams() editArgs }

// toolReply is what a handler produces: a payload merged into the
// envelope, or an image, or a refusal.
type toolReply struct {
	payload map[string]any
	changed []string
	png     []byte
	raw     *mcp.CallToolResult
}

// addTool registers one tool whose handler runs on the game loop. Mutating
// tools are refused while a dialog is open or a stage drag is in progress,
// and honour expect_generation.
func addTool[In any](s *mcpServer, name, desc string, mutating bool, h func(In) (toolReply, error)) {
	ro := !mutating
	t := &mcp.Tool{
		Name:        name,
		Description: desc,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: ro, IdempotentHint: ro},
	}
	mcp.AddTool(s.server, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		var (
			reply toolReply
			herr  error
			env   map[string]any
		)
		err := s.call(ctx, func() {
			m := s.model
			if mutating && (m.DialogOpen() || m.StageDragOpen()) {
				herr = refuse("busy", "a file dialog is open or the author is mid-drag; retry shortly")
				return
			}
			if e, ok := any(in).(edited); ok {
				p := e.editParams()
				if p.ExpectGeneration != nil && *p.ExpectGeneration != m.DocGeneration() {
					herr = refuse(fmt.Sprintf("document generation is %d, not %d", m.DocGeneration(), *p.ExpectGeneration),
						"the document changed since you last read it; inspect again before writing")
					return
				}
				if p.Target != "" {
					if err := s.selectAddress(p.Target); err != nil {
						herr = err
						return
					}
				}
			}
			reply, herr = h(in)
			// The envelope reads the Model (focus, generation, validation
			// problems, which re-serializes the machine), so it has to be
			// built here, on the game loop, and not on the transport's
			// goroutine after call returns.
			if herr == nil && reply.raw == nil {
				env = s.envelope(reply.payload, reply.changed)
			}
		})
		if err != nil {
			return refusalResult(err), nil, nil
		}
		if herr != nil {
			return refusalResult(herr), nil, nil
		}
		if reply.raw != nil {
			return reply.raw, nil, nil
		}
		res := jsonText(env)
		if reply.png != nil {
			res.Content = append(res.Content, &mcp.ImageContent{Data: reply.png, MIMEType: "image/png"})
		}
		return res, nil, nil
	})
}

// ---- resources ----

func (s *mcpServer) registerResources() {
	read := func(ctx context.Context, uri string, f func() (any, error)) (*mcp.ReadResourceResult, error) {
		var (
			v   any
			err error
		)
		if cerr := s.call(ctx, func() { v, err = f() }); cerr != nil {
			return nil, cerr
		}
		if err != nil {
			return nil, err
		}
		b, _ := json.Marshal(v)
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: uri, MIMEType: "application/json", Text: string(b),
		}}}, nil
	}
	s.server.AddResource(&mcp.Resource{
		URI: "lottie://focus", Name: "focus", MIMEType: "application/json",
		Description: "the shared selection: stage, playhead, selected address and its form",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return read(ctx, req.Params.URI, func() (any, error) {
			return map[string]any{"focus": s.focus(), "form": s.formPayload()}, nil
		})
	})
	s.server.AddResource(&mcp.Resource{
		URI: "lottie://problems", Name: "problems", MIMEType: "application/json",
		Description: "validation findings for the open bundle",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return read(ctx, req.Params.URI, func() (any, error) { return s.validation(), nil })
	})
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "lottie://clip/{id}.json", Name: "clip", MIMEType: "application/json",
		Description: "the raw Lottie document of one clip",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := strings.TrimSuffix(strings.TrimPrefix(req.Params.URI, "lottie://clip/"), ".json")
		return read(ctx, req.Params.URI, func() (any, error) {
			data, ok := s.model.Bundle().AnimationJSON(id)
			if !ok {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			return json.RawMessage(data), nil
		})
	})
	s.server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "lottie://machine/{id}.json", Name: "machine", MIMEType: "application/json",
		Description: "one state machine document",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id := strings.TrimSuffix(strings.TrimPrefix(req.Params.URI, "lottie://machine/"), ".json")
		return read(ctx, req.Params.URI, func() (any, error) {
			s.model.syncMachine()
			sm, err := s.model.Bundle().StateMachine(id)
			if err != nil {
				return nil, mcp.ResourceNotFoundError(req.Params.URI)
			}
			return sm, nil
		})
	})
}

// sortedKeys is a small helper for deterministic listings.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
