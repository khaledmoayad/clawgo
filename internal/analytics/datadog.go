// Package analytics provides batched event logging to Datadog HTTP intake
// for analytics. It matches the TypeScript services/analytics/datadog.ts
// with batched POSTs, flush-on-interval, flush-on-batch-size, and
// flush-on-shutdown behavior.
package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/khaledmoayad/clawgo/internal/app"
)

const (
	// datadogEndpoint is the Datadog HTTP intake API endpoint.
	// Matches the TS constant in services/analytics/datadog.ts.
	datadogEndpoint = "https://http-intake.logs.us5.datadoghq.com/api/v2/logs"

	// datadogToken is the client-side Datadog token (not a secret).
	// Matches the TS constant exactly.
	datadogToken = "pubbbf48e6d78dae54bceaa4acf463299bf"

	// flushInterval is how often batched events are flushed to Datadog.
	flushInterval = 15 * time.Second

	// maxBatchSize triggers an immediate flush when reached.
	maxBatchSize = 100

	// networkTimeout is the HTTP client timeout for Datadog requests.
	networkTimeout = 5 * time.Second
)

// DatadogLog is a single log entry sent to the Datadog HTTP intake API.
type DatadogLog struct {
	DDSource string `json:"ddsource"`
	DDTags   string `json:"ddtags"`
	Message  string `json:"message"`
	Service  string `json:"service"`
	Hostname string `json:"hostname"`
}

// DatadogLogger batches analytics events and flushes them to the Datadog
// HTTP intake API. Events are flushed when the batch reaches maxBatchSize,
// every flushInterval, or on Shutdown.
type DatadogLogger struct {
	mu           sync.Mutex
	batch        []DatadogLog
	timer        *time.Timer
	client       *http.Client
	enabled      bool
	shutdownOnce sync.Once
}

// NewDatadogLogger creates a new Datadog event logger. When enabled is false,
// Track calls are no-ops and no HTTP requests are made. When enabled is true,
// a graceful shutdown cleanup is registered via app.RegisterCleanup.
func NewDatadogLogger(enabled bool) *DatadogLogger {
	d := &DatadogLogger{
		client: &http.Client{
			Timeout: networkTimeout,
		},
		enabled: enabled,
	}

	if enabled {
		app.RegisterCleanup(d.Shutdown)
	}

	return d
}

// Track records an analytics event with optional properties. If the logger
// is disabled, this is a no-op. Events are batched and flushed when the
// batch size threshold is reached or on the flush interval timer.
func (d *DatadogLogger) Track(event string, props map[string]interface{}) {
	if !d.enabled {
		return
	}

	entry := DatadogLog{
		DDSource: "go",
		Service:  "claude-code",
		Hostname: "claude-code",
		Message:  event,
		DDTags:   buildTags(event, props),
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.batch = append(d.batch, entry)

	if len(d.batch) >= maxBatchSize {
		d.flushLocked()
		return
	}

	// Schedule flush timer if not already running
	if d.timer == nil {
		d.timer = time.AfterFunc(flushInterval, func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.timer = nil
			if len(d.batch) > 0 {
				d.flushLocked()
			}
		})
	}
}

// Shutdown flushes any remaining batched events and stops the timer.
// It is safe to call multiple times -- subsequent calls are no-ops.
func (d *DatadogLogger) Shutdown() {
	d.shutdownOnce.Do(func() {
		d.mu.Lock()
		if d.timer != nil {
			d.timer.Stop()
			d.timer = nil
		}
		if len(d.batch) > 0 {
			d.flushLocked()
		}
		d.mu.Unlock()
	})
}

// flushLocked copies the current batch and sends it to Datadog in a
// background goroutine. Must be called with d.mu held. The batch is
// reset immediately -- the goroutine works with its own copy.
func (d *DatadogLogger) flushLocked() {
	if len(d.batch) == 0 {
		return
	}

	// Copy batch for async send
	toSend := make([]DatadogLog, len(d.batch))
	copy(toSend, d.batch)
	d.batch = d.batch[:0]

	// Stop and clear timer since we're flushing now
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	go d.send(toSend)
}

// send POSTs a batch of log entries to the Datadog HTTP intake API.
// Errors are logged but not retried (best-effort, matching TS behavior).
func (d *DatadogLogger) send(logs []DatadogLog) {
	body, err := json.Marshal(logs)
	if err != nil {
		return // silently drop malformed batches
	}

	req, err := http.NewRequest(http.MethodPost, datadogEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", datadogToken)

	resp, err := d.client.Do(req)
	if err != nil {
		// Best-effort: log errors but do not retry
		return
	}
	defer resp.Body.Close()
}

// buildTags creates a comma-separated tag string from an event name and
// properties map. Format: "event:eventName,key1:val1,key2:val2".
// Keys are sorted for deterministic output.
func buildTags(event string, props map[string]interface{}) string {
	parts := []string{fmt.Sprintf("event:%s", event)}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%v", k, props[k]))
	}

	return strings.Join(parts, ",")
}
