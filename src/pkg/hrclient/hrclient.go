// Package hrclient posts batches of punches to the ERP HR service's device
// ingestion endpoint. The request/response shapes here mirror the HR service's
// devicestructs.IngestLogsRequest and IngestResultResponse exactly, so the
// contract stays in lock-step with the other side.
package hrclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LogEntry is one punch in the batch. Field tags match HR's DeviceLogEntry.
type LogEntry struct {
	EmployeeNumber string   `json:"employeeNumber"`
	LogType        string   `json:"logType"` // IN | OUT
	EventTime      string   `json:"eventTime"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
}

// ingestRequest matches HR's IngestLogsRequest.
type ingestRequest struct {
	Logs []LogEntry `json:"logs"`
}

// IngestResult mirrors HR's IngestResultResponse (the Data field of the answer).
type IngestResult struct {
	Total     int `json:"total"`
	Accepted  int `json:"accepted"`
	Ignored   int `json:"ignored"`
	Failed    int `json:"failed"`
	Duplicate int `json:"duplicate"`
}

// answer mirrors HR's AnswerResponse envelope with a typed Data payload.
type answer struct {
	State     string       `json:"state"`
	Message   string       `json:"message"`
	IsSuccess bool         `json:"isSuccess"`
	Data      IngestResult `json:"data"`
}

// Response is what Send returns to the forwarder.
type Response struct {
	HTTPStatus int
	IsSuccess  bool // HR accepted and durably stored the batch (HTTP 200 + isSuccess)
	Message    string
	Result     IngestResult // per-batch counts (nil-safe zero value if absent)
	// Auth reports a credential problem (401/403). The forwarder should stop
	// retrying this device until its credential is fixed, rather than hammering.
	Auth bool
}

// Client posts to a fixed HR ingest URL.
type Client struct {
	url  string
	http *http.Client
}

// New builds a client. insecureTLS allows self-signed HR certs (LAN/testing).
func New(url string, timeout time.Duration, insecureTLS bool) *Client {
	tr := &http.Transport{
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     90 * time.Second,
	}
	if insecureTLS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 — opt-in for LAN
	}
	return &Client{url: url, http: &http.Client{Timeout: timeout, Transport: tr}}
}

// Send delivers a batch under one device's credential. A returned error means
// the request could not be completed (network/transport) and should be retried;
// a non-error Response with IsSuccess=false is a definitive HR-side rejection.
func (c *Client) Send(ctx context.Context, deviceID, deviceKey string, logs []LogEntry) (*Response, error) {
	body, err := json.Marshal(ingestRequest{Logs: logs})
	if err != nil {
		return nil, fmt.Errorf("hrclient: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("hrclient: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", deviceID)
	req.Header.Set("X-Device-Key", deviceKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hrclient: post: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	out := &Response{HTTPStatus: resp.StatusCode}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		out.Auth = true
		out.Message = "device credential rejected by HR (" + resp.Status + ")"
		return out, nil
	}

	var a answer
	if err := json.Unmarshal(raw, &a); err != nil {
		// A 200 with an unparseable body is treated as a transient failure.
		if resp.StatusCode == http.StatusOK {
			return nil, fmt.Errorf("hrclient: unreadable 200 response: %s", snippet(raw))
		}
		out.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet(raw))
		return out, nil
	}

	out.Message = a.Message
	out.Result = a.Data
	out.IsSuccess = resp.StatusCode == http.StatusOK && a.IsSuccess
	return out, nil
}

func snippet(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}
