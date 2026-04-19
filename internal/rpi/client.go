package rpi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.httpClient = c }
}

func WithLogger(l *slog.Logger) Option {
	return func(cl *Client) { cl.logger = l }
}

// NewClient builds a client rooted at baseURL (e.g. "http://192.168.1.128:12800").
// Scheme and host are required; a trailing slash is tolerated.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base url must include scheme and host, got %q", baseURL)
	}
	cleaned := strings.TrimRight(parsed.String(), "/")
	client := &Client{
		baseURL:    cleaned,
		httpClient: &http.Client{Timeout: defaultTimeout},
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

func (c *Client) Install(parentCtx context.Context, req InstallRequest) (*InstallResponse, error) {
	if err := validateInstall(req); err != nil {
		return nil, err
	}
	var resp InstallResponse
	if err := c.post(parentCtx, "/api/install", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) IsExists(parentCtx context.Context, titleID string) (*IsExistsResponse, error) {
	var resp IsExistsResponse
	if err := c.post(parentCtx, "/api/is_exists", map[string]string{"title_id": titleID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UninstallGame(parentCtx context.Context, titleID string) error {
	return c.post(parentCtx, "/api/uninstall_game", map[string]string{"title_id": titleID}, nil)
}

func (c *Client) UninstallPatch(parentCtx context.Context, titleID string) error {
	return c.post(parentCtx, "/api/uninstall_patch", map[string]string{"title_id": titleID}, nil)
}

func (c *Client) UninstallAC(parentCtx context.Context, contentID string) error {
	return c.post(parentCtx, "/api/uninstall_ac", map[string]string{"content_id": contentID}, nil)
}

func (c *Client) UninstallTheme(parentCtx context.Context, contentID string) error {
	return c.post(parentCtx, "/api/uninstall_theme", map[string]string{"content_id": contentID}, nil)
}

func (c *Client) StartTask(parentCtx context.Context, taskID int64) error {
	return c.post(parentCtx, "/api/start_task", taskIDBody(taskID), nil)
}

func (c *Client) StopTask(parentCtx context.Context, taskID int64) error {
	return c.post(parentCtx, "/api/stop_task", taskIDBody(taskID), nil)
}

func (c *Client) PauseTask(parentCtx context.Context, taskID int64) error {
	return c.post(parentCtx, "/api/pause_task", taskIDBody(taskID), nil)
}

func (c *Client) ResumeTask(parentCtx context.Context, taskID int64) error {
	return c.post(parentCtx, "/api/resume_task", taskIDBody(taskID), nil)
}

func (c *Client) UnregisterTask(parentCtx context.Context, taskID int64) error {
	return c.post(parentCtx, "/api/unregister_task", taskIDBody(taskID), nil)
}

func (c *Client) GetTaskProgress(parentCtx context.Context, taskID int64) (*TaskProgress, error) {
	var resp TaskProgress
	if err := c.post(parentCtx, "/api/get_task_progress", taskIDBody(taskID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) FindTask(parentCtx context.Context, contentID string, subType SubType) (*FindTaskResponse, error) {
	body := map[string]any{"content_id": contentID, "sub_type": int(subType)}
	var resp FindTaskResponse
	if err := c.post(parentCtx, "/api/find_task", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) post(parentCtx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal %s body: %w", path, err)
	}
	req, err := http.NewRequestWithContext(parentCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}
	defer func() {
		// Skip: body already consumed, close error is not actionable.
		_ = resp.Body.Close()
	}()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s response: %w", path, err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("call %s: http status %d: %s", path, resp.StatusCode, truncate(raw, 200))
	}
	cleaned := sanitizeHexNumbers(raw)
	var envelope statusEnvelope
	if err := json.Unmarshal(cleaned, &envelope); err != nil {
		return fmt.Errorf("decode %s envelope: %w", path, err)
	}
	if envelope.Status != "success" {
		return &APIError{Message: extractErrorMessage(envelope.Error), ErrorCode: envelope.ErrorCode}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(cleaned, out); err != nil {
		return fmt.Errorf("decode %s payload: %w", path, err)
	}
	return nil
}

// extractErrorMessage parses a failure envelope's `error` field when it is
// encoded as a JSON string. Non-string shapes (integer, null, missing) are
// skipped on purpose: the caller still reports ErrorCode.
func extractErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var message string
	if err := json.Unmarshal(raw, &message); err != nil {
		return ""
	}
	return message
}

func validateInstall(req InstallRequest) error {
	switch req.Type {
	case PackageTypeDirect:
		if len(req.Packages) == 0 {
			return fmt.Errorf("install: packages required for type %q", req.Type)
		}
		if req.URL != "" {
			return fmt.Errorf("install: url must be empty for type %q", req.Type)
		}
	case PackageTypeRefPkgURL:
		if req.URL == "" {
			return fmt.Errorf("install: url required for type %q", req.Type)
		}
		if len(req.Packages) > 0 {
			return fmt.Errorf("install: packages must be empty for type %q", req.Type)
		}
	default:
		return fmt.Errorf("install: unknown type %q", req.Type)
	}
	return nil
}

func taskIDBody(taskID int64) map[string]int64 {
	return map[string]int64{"task_id": taskID}
}

func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}
