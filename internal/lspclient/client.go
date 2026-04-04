package lspclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/khaledmoayad/clawgo/internal/app"
)

// Client is a lightweight LSP client that communicates with a language server
// process over JSON-RPC 2.0 via stdio. It handles the Content-Length framing
// protocol and the initialize handshake.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	nextID  atomic.Int64
	pending map[int64]chan json.RawMessage
	mu      sync.Mutex

	capabilities json.RawMessage
	initialized  bool

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewClient spawns a language server process and returns a Client connected
// to its stdio. The server is started with the given command and arguments.
// A cleanup function is registered via app.RegisterCleanup to kill the
// server process on shutdown.
func NewClient(ctx context.Context, serverCmd string, args ...string) (*Client, error) {
	clientCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(clientCtx, serverCmd, args...)

	// On Linux, set Pdeathsig to ensure the child is killed if the parent dies.
	// This prevents orphaned language server processes.
	if runtime.GOOS == "linux" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		}
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("lspclient: stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("lspclient: stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("lspclient: start server %q: %w", serverCmd, err)
	}

	c := &Client{
		cmd:     cmd,
		stdin:   stdinPipe,
		stdout:  bufio.NewReaderSize(stdoutPipe, 64*1024),
		pending: make(map[int64]chan json.RawMessage),
		ctx:     clientCtx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	// Start background read loop
	go c.readLoop()

	// Register cleanup to kill the process on app shutdown
	app.RegisterCleanup(func() {
		_ = c.Close()
	})

	return c, nil
}

// Initialize performs the LSP initialize handshake with the server.
// It sends the initialize request, waits for the response, stores
// server capabilities, and sends the "initialized" notification.
func (c *Client) Initialize(ctx context.Context, rootURI string) (*InitializeResult, error) {
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocClientCap{
				PublishDiagnostics: &PublishDiagCap{
					RelatedInformation: true,
				},
			},
		},
	}

	raw, err := c.Request(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("lspclient: initialize: %w", err)
	}

	var result InitializeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("lspclient: unmarshal initialize result: %w", err)
	}

	c.capabilities = result.Capabilities
	c.initialized = true

	// Send "initialized" notification
	if err := c.Notify("initialized", struct{}{}); err != nil {
		return nil, fmt.Errorf("lspclient: initialized notification: %w", err)
	}

	return &result, nil
}

// Request sends a JSON-RPC request and waits for the response.
func (c *Client) Request(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("lspclient: marshal params: %w", err)
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	// Register pending response channel before sending
	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.writeMessage(msg); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("lspclient: client closed")
	}
}

// Notify sends a JSON-RPC notification (no response expected).
func (c *Client) Notify(method string, params interface{}) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("lspclient: marshal params: %w", err)
	}

	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
	}

	return c.writeMessage(msg)
}

// Initialized returns whether the initialize handshake has completed.
func (c *Client) Initialized() bool {
	return c.initialized
}

// Capabilities returns the server capabilities from the initialize response.
func (c *Client) Capabilities() json.RawMessage {
	return c.capabilities
}

// writeMessage marshals the message and writes it with Content-Length framing.
func (c *Client) writeMessage(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("lspclient: marshal message: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := io.WriteString(c.stdin, header); err != nil {
		return fmt.Errorf("lspclient: write header: %w", err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		return fmt.Errorf("lspclient: write body: %w", err)
	}

	return nil
}

// readMessage reads a single Content-Length framed message from stdout.
func (c *Client) readMessage() (json.RawMessage, error) {
	// Read headers until empty line
	contentLength := -1
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("lspclient: read header: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("lspclient: invalid Content-Length %q: %w", val, err)
			}
		}
		// Ignore other headers (e.g., Content-Type)
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("lspclient: missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("lspclient: read body: %w", err)
	}

	return json.RawMessage(body), nil
}

// readLoop continuously reads messages from stdout and dispatches responses.
func (c *Client) readLoop() {
	defer close(c.done)

	for {
		raw, err := c.readMessage()
		if err != nil {
			// Server disconnected or read error; exit loop
			return
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		// Dispatch response to pending request
		if msg.IsResponse() && msg.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()

			if ok {
				if msg.Error != nil {
					// Send error as nil result; caller should check
					ch <- nil
				} else {
					ch <- msg.Result
				}
			}
		}
		// Notifications and requests from server are currently ignored
		// (diagnostics etc. would be handled by a notification handler)
	}
}

// Close performs a graceful LSP shutdown: sends "shutdown" request,
// then "exit" notification, and kills the process if still running.
func (c *Client) Close() error {
	if c.cancel != nil {
		defer c.cancel()
	}

	// Try graceful shutdown
	if c.initialized {
		ctx := context.Background()
		// Send shutdown request (ignore errors, server may already be gone)
		_, _ = c.Request(ctx, "shutdown", nil)
		// Send exit notification
		_ = c.Notify("exit", nil)
	}

	// Close stdin to signal EOF to server
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	// Kill process if still running
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}

	return nil
}
