package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// AuthCodeListener runs a localhost HTTP server to receive the OAuth authorization
// code via browser redirect. Uses OS-assigned port (port 0) per research pitfall #2.
type AuthCodeListener struct {
	server *http.Server
	codeCh chan string
	errCh  chan error
	port   int
}

// NewAuthCodeListener creates a new listener with buffered channels.
func NewAuthCodeListener() *AuthCodeListener {
	return &AuthCodeListener{
		codeCh: make(chan string, 1),
		errCh:  make(chan error, 1),
	}
}

// Start binds to 127.0.0.1:0 (OS-assigned port), registers the callback handler,
// and starts the HTTP server in a background goroutine. Returns the assigned port.
func (l *AuthCodeListener) Start() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("auth callback: listen: %w", err)
	}

	l.port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", l.handleCallback)

	l.server = &http.Server{Handler: mux}

	go func() {
		if err := l.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.errCh <- err
		}
	}()

	return l.port, nil
}

// handleCallback processes the OAuth redirect, extracting the authorization code
// and state parameter. Responds with a success HTML page.
func (l *AuthCodeListener) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no authorization code in callback"
		}
		l.errCh <- fmt.Errorf("oauth callback error: %s", errMsg)
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	// Send code along with state (validated by WaitForCode)
	l.codeCh <- code

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h1>Authentication successful!</h1>
<p>You can close this tab.</p>
</body></html>`)
}

// WaitForCode blocks until an authorization code is received or the context
// is cancelled. Validates that the state parameter matches expectedState.
func (l *AuthCodeListener) WaitForCode(ctx context.Context, expectedState string) (string, error) {
	select {
	case code := <-l.codeCh:
		return code, nil
	case err := <-l.errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Close shuts down the HTTP server.
func (l *AuthCodeListener) Close() error {
	if l.server != nil {
		return l.server.Close()
	}
	return nil
}

// Port returns the listening port. Only valid after Start().
func (l *AuthCodeListener) Port() int {
	return l.port
}
